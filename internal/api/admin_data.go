package api

import (
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/backup"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/features/datacheck"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

func (h *handlers) registerAdminData(g *echo.Group) {
	d := g.Group("/data", middleware.RequireRole(auth.RoleAdmin))
	d.GET("", h.AdminData)
	d.GET("/backup", h.AdminDataBackup)
	d.POST("/restore", h.AdminDataRestore, echomw.BodyLimit("256M"))
	d.GET("/check", h.AdminDataCheck)
	d.GET("/settings/export", h.AdminSettingsExport)
	d.POST("/settings/import", h.AdminSettingsImport)
	d.GET("/backups/:name", h.AdminDataBackupFile)
	d.POST("/backups/:name/restore", h.AdminDataRestoreFile)
}

func (h *handlers) AdminData(c echo.Context) error {
	folder := h.d.Cfg.BackupFolder()
	files, err := backup.ListBackups(folder)
	if err != nil {
		return err
	}
	page := DataPage{
		Folder:        folder,
		AutoBackups:   h.d.Cfg.BackupDir != "",
		IntervalHours: int(h.d.Cfg.BackupInterval / time.Hour),
		Keep:          h.d.Cfg.BackupKeep,
		FileDataDir:   h.d.Cfg.FileDataDir,
		Groups:        []DataGroup{},
		Backups:       make([]BackupFileRow, 0, len(files)),
	}
	for _, g := range plugin.CollectDataGroups() {
		page.Groups = append(page.Groups, DataGroup{Key: g.Key, Name: g.Name})
	}
	for _, f := range files {
		page.Backups = append(page.Backups, BackupFileRow{Name: f.Name, Size: f.Size, ModifiedAt: f.ModifiedAt,
			BeforeRestore: strings.HasSuffix(f.Name, "-"+backup.BeforeRestoreTag+".json.gz")})
	}
	return c.JSON(http.StatusOK, page)
}

// AdminDataBackup streams a backup: full with no params; ?groups= (repeatable)
// keeps only those plugin groups; ?setupOnly=1 drops the trading history and
// zeroes contact balances.
func (h *handlers) AdminDataBackup(c echo.Context) error {
	ctx := c.Request().Context()
	now := time.Now()
	opts := backup.DumpOpts{}
	qp := c.QueryParams()
	_, choose := qp["groups"]
	setupOnly := c.QueryParam("setupOnly") == "1"
	if choose || setupOnly {
		groups := plugin.CollectDataGroups()
		keep := []string{}
		for _, k := range qp["groups"] { // "groups=" alone = none of them
			if k != "" {
				keep = append(keep, k)
			}
		}
		if !choose {
			keep = make([]string, 0, len(groups))
			for _, g := range groups {
				keep = append(keep, g.Key)
			}
		}
		all, err := backup.AllTables(ctx, h.d.DB)
		if err != nil {
			return err
		}
		exclude := backup.ExcludeFor(groups, keep)
		if setupOnly {
			exclude = append(exclude, backup.SetupOnlyExclude(backup.CoreTables(all, groups))...)
			opts.Zero = backup.SetupZeroColumns
		}
		opts.Exclude, opts.Groups = exclude, keep
		opts.Partial = len(keep) < len(groups) || setupOnly
	}
	name := backup.FileName(h.shopName(c), now, opts.Partial)
	c.Response().Header().Set(echo.HeaderContentType, "application/gzip")
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="`+name+`"`)
	c.Response().WriteHeader(http.StatusOK)
	// Headers are sent; a failure here can only be logged.
	if err := backup.Dump(ctx, h.d.DB, now.Format(time.RFC3339), c.Response().Writer, opts); err != nil {
		c.Logger().Errorf("backup failed: %v", err)
		return nil
	}
	h.audit(c, audit.ActionBackup, "system", "", "downloaded a backup "+name)
	return nil
}

func (h *handlers) shopName(c echo.Context) string {
	if s, err := h.d.Settings.Get(c.Request().Context()); err == nil {
		return s.ShopName
	}
	return ""
}

func (h *handlers) backupPath(c echo.Context) (string, error) {
	p, err := backup.SafePath(h.d.Cfg.BackupFolder(), c.Param("name"))
	if err != nil {
		return "", apperr.BadRequest("not a backup file name")
	}
	if _, err := os.Stat(p); err != nil {
		return "", apperr.NotFound("backup file")
	}
	return p, nil
}

func (h *handlers) AdminDataBackupFile(c echo.Context) error {
	p, err := h.backupPath(c)
	if err != nil {
		return err
	}
	return c.Attachment(p, c.Param("name"))
}

func (h *handlers) AdminDataRestore(c echo.Context) error {
	fh, err := c.FormFile("backup")
	if err != nil {
		return apperr.BadRequest("choose a backup file (.json.gz) to restore")
	}
	f, err := fh.Open()
	if err != nil {
		return apperr.BadRequest("cannot read the uploaded file")
	}
	defer f.Close()
	return h.restoreFrom(c, f, fh.Filename)
}

func (h *handlers) AdminDataRestoreFile(c echo.Context) error {
	p, err := h.backupPath(c)
	if err != nil {
		return err
	}
	f, err := os.Open(p)
	if err != nil {
		return apperr.BadRequest("could not open that backup file")
	}
	defer f.Close()
	return h.restoreFrom(c, f, c.Param("name"))
}

// restoreFrom takes a safety copy of the current data first (refusing to go on
// without one — it is the only undo), restores, ends the session (the users
// table was replaced, so this login may now mean someone else) and reports.
func (h *handlers) restoreFrom(c echo.Context, r io.Reader, label string) error {
	ctx := c.Request().Context()
	who := middleware.CurrentUserName(c)
	arc, err := backup.ReadArchive(r)
	if err != nil {
		return apperr.Validation("that file can't be restored, nothing was changed: " + err.Error())
	}
	safety, err := backup.WriteSnapshot(ctx, h.d.DB, h.d.Cfg.BackupFolder(), time.Now(), backup.BeforeRestoreTag)
	if err != nil {
		return apperr.Validation("could not save a copy of the current data first, so nothing was restored: " + err.Error())
	}
	rep, err := backup.RestoreArchive(ctx, h.d.DB, arc)
	if err != nil {
		return apperr.Validation("restore failed, nothing was changed: " + err.Error())
	}
	h.audit(c, audit.ActionRestore, "system", "", "restored the data from "+label+" (by "+who+")")
	h.clearCookie(c)
	res := RestoreResult{
		Report: RestoreReport{Tables: rep.Tables, Rows: rep.Rows,
			SkippedTables: orEmpty(rep.SkippedTables), DroppedColumns: orEmpty(rep.DroppedColumns)},
		SafetyCopy: baseName(safety),
		Check:      h.dataCheck(c),
	}
	return c.JSON(http.StatusOK, res)
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func baseName(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

// checkFindings runs the data check (stored data older karots-pos versions
// wrote wrongly — see internal/features/datacheck).
func (h *handlers) checkFindings(c echo.Context) ([]CheckFinding, error) {
	found, err := datacheck.Run(c.Request().Context(), h.d.DB)
	if err != nil {
		return nil, err
	}
	out := make([]CheckFinding, 0, len(found))
	for _, f := range found {
		rows := make([]CheckFindingRow, len(f.Sample))
		for i, r := range f.Sample {
			rows[i] = CheckFindingRow{Ref: r.Ref, Detail: r.Detail}
		}
		out = append(out, CheckFinding{Key: f.Key, Title: f.Title, Explain: f.Explain, Count: f.Count, Sample: rows})
	}
	return out, nil
}

// dataCheck is checkFindings for after a restore: a failed check must not hide
// the restore's success, so it logs and reports nothing.
func (h *handlers) dataCheck(c echo.Context) []CheckFinding {
	out, err := h.checkFindings(c)
	if err != nil {
		c.Logger().Errorf("data check: %v", err)
		return []CheckFinding{}
	}
	return out
}

func (h *handlers) AdminDataCheck(c echo.Context) error {
	out, err := h.checkFindings(c)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out)
}
