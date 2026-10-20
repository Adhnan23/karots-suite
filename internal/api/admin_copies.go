package api

import (
	"errors"
	"net/http"
	osuser "os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/backup"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/folders"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

// Backup copies (M8c2): where the automatic backups are also copied to — the
// owner's copy folder (USB / share) and, in paying shops, Karots cloud.
func (h *handlers) registerAdminCopies(d *echo.Group) {
	d.GET("/copies", h.AdminCopies)
	d.PUT("/copies/folder", h.AdminCopyFolderSave)
	d.DELETE("/copies/folder", h.AdminCopyFolderClear)
	d.PUT("/copies/cloud", h.AdminCopyCloudSwitch)
	d.POST("/copies/:kind/run", h.AdminCopyRun)
	d.GET("/copies/:kind/backups", h.AdminCopyBackups)
	d.POST("/copies/:kind/backups/:name/fetch", h.AdminCopyFetch)
	d.GET("/folders", h.AdminFolders)
	d.POST("/folders", h.AdminFolderNew)
}

// AdminFolders is the copy folder's "Choose…": the folders on the server's PC
// (names only, no files), for an admin.
func (h *handlers) AdminFolders(c echo.Context) error {
	user, home := "", ""
	if u, err := osuser.Current(); err == nil {
		user, home = u.Username, u.HomeDir
	}
	l, err := folders.List(c.QueryParam("path"), folders.Roots(user, home))
	if err != nil {
		return apperr.BadRequest(err.Error())
	}
	out := FolderListing{Path: l.Path, Parent: l.Parent, Folders: l.Folders, Roots: make([]FolderRoot, 0, len(l.Roots))}
	for _, r := range l.Roots {
		out.Roots = append(out.Roots, FolderRoot{Name: r.Name, Path: r.Path})
	}
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) AdminFolderNew(c echo.Context) error {
	var req NewFolderRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request")
	}
	p, err := folders.Make(req.Path, req.Name)
	if err != nil {
		return apperr.Validation(err.Error())
	}
	return c.JSON(http.StatusOK, FolderListing{Path: p, Folders: []string{}, Roots: []FolderRoot{}})
}

// copier is the API's own copier; the backup package keeps it and the
// server's background pass from copying at the same time.
func (h *handlers) copier() *backup.Copier {
	return backup.NewCopier(h.d.DB, h.d.Cfg.BackupFolder(), h.d.Cfg.BackupKeep, h.d.Cfg.Cloud)
}

func (h *handlers) copyRow(cp *backup.Copier, r backup.Copy) BackupCopyRow {
	return BackupCopyRow{Kind: r.Kind, Enabled: r.Enabled, Path: r.Path, LastCopiedAt: r.LastCopiedAt,
		LastTriedAt: r.LastTriedAt, LastFile: r.LastFile, LastError: r.LastError, Waiting: cp.Waiting(r)}
}

func (h *handlers) AdminCopies(c echo.Context) error {
	rows, err := backup.Copies(c.Request().Context(), h.d.DB)
	if err != nil {
		return err
	}
	cp := h.copier()
	out := make([]BackupCopyRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, h.copyRow(cp, r))
	}
	return c.JSON(http.StatusOK, out)
}

// copyOf is the place named in the URL; 404 when it isn't set up.
func (h *handlers) copyOf(c echo.Context, kind string) (*backup.Copy, error) {
	if kind != backup.KindFolder && kind != backup.KindCloud {
		return nil, apperr.NotFound("backup copy")
	}
	r, err := backup.CopyOf(c.Request().Context(), h.d.DB, kind)
	if err != nil {
		return nil, err
	}
	if r == nil || (kind == backup.KindFolder && r.Path == "") {
		return nil, apperr.NotFound("backup copy")
	}
	return r, nil
}

func (h *handlers) copyJSON(c echo.Context, kind string) error {
	r, err := h.copyOf(c, kind)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.copyRow(h.copier(), *r))
}

// insideFolder reports whether path is dir or inside it.
func insideFolder(path, dir string) bool {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		path, abs = strings.ToLower(path), strings.ToLower(abs)
	}
	rel, err := filepath.Rel(abs, filepath.Clean(path))
	return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))))
}

func (h *handlers) AdminCopyFolderSave(c echo.Context) error {
	var req CopyFolderRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request")
	}
	path := strings.TrimSpace(req.Path)
	if path == "" || !filepath.IsAbs(path) {
		return apperr.Validation("Type the full folder path, like E:\\Karots copies or /media/usb/Karots copies")
	}
	path = filepath.Clean(path)
	// A copy inside the backups folder would copy into itself and prune the shop's own backups.
	if insideFolder(path, h.d.Cfg.BackupFolder()) {
		return &apperr.AppError{Code: "BACKUP_FOLDER_IS_COPY", Status: http.StatusBadRequest,
			Message: "Pick a different folder — that is where the backups already are"}
	}
	cp := h.copier()
	if err := cp.TestFolder(path); err != nil {
		return apperr.Validation(err.Error())
	}
	ctx := c.Request().Context()
	if err := backup.SaveFolder(ctx, h.d.DB, path, req.Enabled); err != nil {
		return err
	}
	h.audit(c, audit.ActionSettings, "system", "", "set the backup copy folder to "+path)
	return h.copyJSON(c, backup.KindFolder)
}

func (h *handlers) AdminCopyFolderClear(c echo.Context) error {
	if err := backup.ClearFolder(c.Request().Context(), h.d.DB); err != nil {
		return err
	}
	h.audit(c, audit.ActionSettings, "system", "", "cleared the backup copy folder")
	return c.NoContent(http.StatusNoContent)
}

// AdminCopyCloudSwitch: any admin may switch Karots cloud off; only Karots
// (the system user) switches it on — it is a paid service.
func (h *handlers) AdminCopyCloudSwitch(c echo.Context) error {
	var req CopyCloudRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request")
	}
	if _, err := h.copyOf(c, backup.KindCloud); err != nil {
		return err
	}
	if req.Enabled && !middleware.IsSystemUserCtx(c.Request().Context()) {
		return apperr.Forbidden("Ask Karots to switch Karots cloud on")
	}
	if err := backup.SetEnabled(c.Request().Context(), h.d.DB, backup.KindCloud, req.Enabled); err != nil {
		return err
	}
	h.audit(c, audit.ActionSettings, "system", "", map[bool]string{true: "switched Karots cloud backups on", false: "switched Karots cloud backups off"}[req.Enabled])
	return h.copyJSON(c, backup.KindCloud)
}

// AdminCopyRun is Copy now: the row comes back with how it went.
func (h *handlers) AdminCopyRun(c echo.Context) error {
	kind := c.Param("kind")
	if _, err := h.copyOf(c, kind); err != nil {
		return err
	}
	h.copier().CopyNow(c.Request().Context(), kind) //nolint:errcheck // stored on the row
	return h.copyJSON(c, kind)
}

func (h *handlers) AdminCopyBackups(c echo.Context) error {
	r, err := h.copyOf(c, c.Param("kind"))
	if err != nil {
		return err
	}
	files, err := h.copier().Backups(c.Request().Context(), *r)
	if err != nil {
		return apperr.Validation("Can't list that copy: " + err.Error())
	}
	out := make([]BackupFileRow, 0, len(files))
	for _, f := range files {
		out = append(out, BackupFileRow{Name: f.Name, Size: f.Size, ModifiedAt: f.ModifiedAt,
			BeforeRestore: strings.HasSuffix(f.Name, "-"+backup.BeforeRestoreTag+".json.gz")})
	}
	return c.JSON(http.StatusOK, out)
}

// AdminCopyFetch brings a backup from a copy into the backups folder; the SPA
// then restores it with POST /backups/:name/restore.
func (h *handlers) AdminCopyFetch(c echo.Context) error {
	r, err := h.copyOf(c, c.Param("kind"))
	if err != nil {
		return err
	}
	name, err := h.copier().Fetch(c.Request().Context(), *r, c.Param("name"))
	if errors.Is(err, backup.ErrNotListed) {
		return apperr.BadRequest(err.Error())
	}
	if err != nil {
		return apperr.Validation("Couldn't fetch that backup: " + err.Error())
	}
	h.audit(c, audit.ActionRestore, "system", "", "fetched "+name+" from the "+r.Kind+" backup copy")
	return c.JSON(http.StatusOK, FetchedBackup{Name: name})
}

// staleCopies names the switched-on copies with no copy in 24 h (or none yet,
// once one was tried) for the dashboard.
func staleCopies(rows []backup.Copy, now time.Time) []string {
	out := []string{}
	for _, r := range rows {
		if !r.Enabled || (r.Kind == backup.KindFolder && r.Path == "") {
			continue
		}
		stale := r.LastTriedAt != nil
		if r.LastCopiedAt != nil {
			stale = now.Sub(*r.LastCopiedAt) > 24*time.Hour
		}
		if stale {
			out = append(out, map[string]string{backup.KindFolder: "Copy folder", backup.KindCloud: "Karots cloud"}[r.Kind])
		}
	}
	return out
}
