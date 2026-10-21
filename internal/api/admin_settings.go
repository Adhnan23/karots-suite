package api

import (
	"io"
	"net/http"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/escpos"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/lockers"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/printing"
	"karots-suite/internal/receiptimg"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

// registerAdminSettings mounts Setup → Settings (admin + manager, as karots-pos).
func (h *handlers) registerAdminSettings(g *echo.Group) {
	g.GET("/settings", h.AdminSettings)
	g.PUT("/settings", h.AdminSettingsSave)
	g.POST("/settings/logo", h.AdminSettingsLogo, echomw.BodyLimit("8M"))
	g.POST("/settings/logo/clear", h.AdminSettingsLogoClear)
	g.POST("/settings/printer/test", h.AdminSettingsPrinterTest)
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func strPtr(s string) *string {
	s = strings.TrimSpace(s)
	return &s
}

func toShopSettings(s *settings.Settings) ShopSettings {
	out := ShopSettings{
		ShopName: s.ShopName, ShopNameLocal: str(s.ShopNameSi), Address: str(s.Address), Phone: str(s.Phone),
		CurrencyCode: s.CurrencyCode, CurrencySymbol: s.CurrencySymbol, LogoURL: str(s.LogoURL),
		ReceiptFooter: str(s.ReceiptFooter), ReceiptWidth: s.ReceiptWidth, DefaultSaleType: s.DefaultSaleType,
		AskToPrint: s.AskToPrint, PrintAs: s.PrintAs, TaxRegistered: s.TaxRegistered, TaxRegNo: str(s.TaxRegNo),
		OpenCashDrawer: s.OpenCashDrawer, DrawerKickPin: s.DrawerKickPin, AllowUntrackedCash: s.AllowUntrackedCash,
		AllowContinueLastCount: s.AllowContinueLastCount, LowStockAlerts: s.LowStockAlerts, StockTakeEnabled: s.StockTakeEnabled,
		ForcePinChange: s.ForcePinChange, AllowCashierPinChange: s.AllowCashierPinChange, LockTimeoutMinutes: s.LockTimeoutMinutes,
		ReceiptPrinter: s.ReceiptPrinter, LabelPrinter: s.LabelPrinter,
		LabelWidthMM: s.LabelWidthMM, LabelHeightMM: s.LabelHeightMM, LabelGapMM: s.LabelGapMM,
		LabelNameLines: s.LabelNameLines, LabelBottomLines: s.LabelBottomLines,
		LabelNameFont: s.LabelNameFont, LabelBottomFont: s.LabelBottomFont,
		ReorderDefaultDays: s.ReorderDefaultDays, ReorderSafetyDays: s.ReorderSafetyDays,
	}
	if s.DefaultLockerID != nil {
		out.DefaultLockerID = *s.DefaultLockerID
	}
	return out
}

func (h *handlers) AdminSettings(c echo.Context) error { return h.settingsPage(c, true) }

// settingsPage answers the Settings page. Finding the printers asks the OS
// (lpstat / the spooler), which can take seconds, so a save answers without
// them (printers: null) and the page keeps the list it has.
func (h *handlers) settingsPage(c echo.Context, withPrinters bool) error {
	ctx := c.Request().Context()
	s, err := h.d.Settings.Get(ctx)
	if err != nil {
		return err
	}
	page := SettingsPage{Settings: toShopSettings(s), Logo: s.LogoSrc(), LogoUploaded: s.LogoData != "", Lockers: []SettingsLocker{}}
	if withPrinters {
		page.Printers = printing.Queues(ctx)
		if page.Printers == nil {
			page.Printers = []string{}
		}
	}
	if err := h.d.DB.SelectContext(ctx, &page.Lockers, `SELECT id, name FROM lockers WHERE is_active AND kind NOT IN ('bank', 'card') ORDER BY name`); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, page)
}

func (h *handlers) AdminSettingsSave(c echo.Context) error {
	var in ShopSettings
	if err := c.Bind(&in); err != nil {
		return apperr.BadRequest("invalid settings")
	}
	ctx := c.Request().Context()
	if in.DefaultLockerID > 0 {
		l, err := h.t.lockers.Get(ctx, in.DefaultLockerID)
		if err != nil {
			return err
		}
		if !l.IsActive {
			return apperr.Validation("that locker is archived")
		}
		if !lockers.HoldsCash(l.Kind) {
			return apperr.Validation("the till's default locker must hold cash — " + l.Name + " is a bank or card account")
		}
	}
	// Tax is charged only with a registration number (sales.TaxOnSQL): say so
	// here rather than let "registered" with no number quietly charge nothing.
	if in.TaxRegistered && strings.TrimSpace(in.TaxRegNo) == "" {
		return apperr.Validation("enter the VAT registration number — without it no tax is charged")
	}
	up := settings.UpdateInput{
		ShopName: strings.TrimSpace(in.ShopName), ShopNameSi: strPtr(in.ShopNameLocal), Address: strPtr(in.Address),
		Phone: strPtr(in.Phone), CurrencyCode: strings.TrimSpace(in.CurrencyCode), CurrencySymbol: strings.TrimSpace(in.CurrencySymbol),
		LogoURL: strPtr(in.LogoURL), ReceiptFooter: strPtr(in.ReceiptFooter), ReceiptWidth: in.ReceiptWidth,
		TaxRegistered: in.TaxRegistered, TaxRegNo: strPtr(in.TaxRegNo), LowStockAlerts: in.LowStockAlerts,
		AskToPrint: in.AskToPrint, PrintAs: in.PrintAs, ForcePinChange: in.ForcePinChange, AllowCashierPinChange: in.AllowCashierPinChange,
		DefaultSaleType: in.DefaultSaleType, ReceiptPrinter: strings.TrimSpace(in.ReceiptPrinter), LabelPrinter: strings.TrimSpace(in.LabelPrinter),
		LabelWidthMM: in.LabelWidthMM, LabelHeightMM: in.LabelHeightMM, LabelGapMM: in.LabelGapMM,
		LabelNameLines: in.LabelNameLines, LabelBottomLines: in.LabelBottomLines,
		LabelNameFont: in.LabelNameFont, LabelBottomFont: in.LabelBottomFont,
		LockTimeoutMinutes: in.LockTimeoutMinutes, StockTakeEnabled: in.StockTakeEnabled,
		OpenCashDrawer: in.OpenCashDrawer, DrawerKickPin: in.DrawerKickPin,
		AllowUntrackedCash: in.AllowUntrackedCash, DefaultLockerID: in.DefaultLockerID,
		AllowContinueLastCount: in.AllowContinueLastCount,
		ReorderDefaultDays:     in.ReorderDefaultDays, ReorderSafetyDays: in.ReorderSafetyDays,
	}
	if err := c.Validate(&up); err != nil {
		return err
	}
	if _, err := h.d.Settings.Update(ctx, up); err != nil {
		return err
	}
	h.audit(c, audit.ActionSettings, "settings", "", "updated shop settings")
	return h.settingsPage(c, false)
}

// AdminSettingsLogo stores an uploaded image, scaled down to 400px, as a data
// URI in the settings row so it prints and previews offline.
func (h *handlers) AdminSettingsLogo(c echo.Context) error {
	fh, err := c.FormFile("logo")
	if err != nil {
		return apperr.BadRequest("choose an image file")
	}
	f, err := fh.Open()
	if err != nil {
		return apperr.BadRequest("could not read the uploaded file")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 8<<20))
	if err != nil {
		return err
	}
	uri, err := receiptimg.ToDataURI(data, 400)
	if err != nil {
		return apperr.Validation("that file is not a picture we can use — try a PNG or JPEG")
	}
	if err := h.d.Settings.SetLogo(c.Request().Context(), uri); err != nil {
		return err
	}
	h.audit(c, audit.ActionSettings, "settings", "logo", "uploaded a logo")
	return h.settingsPage(c, false)
}

func (h *handlers) AdminSettingsLogoClear(c echo.Context) error {
	if err := h.d.Settings.SetLogo(c.Request().Context(), ""); err != nil {
		return err
	}
	h.audit(c, audit.ActionSettings, "settings", "logo", "removed the logo")
	return h.settingsPage(c, false)
}

// AdminSettingsPrinterTest prints a short slip on the SAVED receipt printer and
// pops the drawer — a one-click hardware check during setup. ?preview=1 answers
// the slip as the paper shows it; an image/png body prints that picture instead.
func (h *handlers) AdminSettingsPrinterTest(c echo.Context) error {
	ctx := c.Request().Context()
	s, err := h.d.Settings.Get(ctx)
	if err != nil {
		return err
	}
	doc := escpos.TestDocument(*s)
	if ok, err := previewSlip(c, s, doc); ok {
		return err
	}
	if job, ok, err := slipImage(c, s); err != nil {
		return err
	} else if ok {
		doc = job
	}
	if strings.TrimSpace(s.ReceiptPrinter) == "" {
		return apperr.Validation("pick and save a receipt printer first")
	}
	if err := escpos.Send(ctx, s.ReceiptPrinter, doc); err != nil {
		return apperr.Validation("the test slip didn't print: " + err.Error())
	}
	escpos.KickDrawer(ctx, *s)
	h.audit(c, audit.ActionSettings, "settings", "", "sent a printer test")
	return c.NoContent(http.StatusNoContent)
}
