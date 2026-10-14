package pharmacy

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

// The plugin's own JSON, mirrored by hand in web/src/plugins/pharmacy/types.ts.
// Quantities are decimal strings, dates YYYY-MM-DD, times ISO 8601.

type FoundDTO struct {
	PatientID  int64  `json:"patientId"` // 0 = a customer, not a patient yet
	CustomerID int64  `json:"customerId"`
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	NIC        string `json:"nic"`
	Allergies  string `json:"allergies"`
}

type PatientDTO struct {
	ID         int64  `json:"id"`
	CustomerID int64  `json:"customerId"`
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	DOB        string `json:"dob"`
	Sex        string `json:"sex"`
	NIC        string `json:"nic"`
	Allergies  string `json:"allergies"`
	Notes      string `json:"notes"`
}

type SuggestDTO struct {
	ProductID int64  `json:"productId"`
	Name      string `json:"name"`
	Stock     string `json:"stock"`
}

type ScriptItemDTO struct {
	ID          int64       `json:"id"`
	ProductID   *int64      `json:"productId"`
	GroupID     *int64      `json:"groupId"`
	Drug        string      `json:"drug"`
	ProductName string      `json:"productName"`
	GroupName   string      `json:"groupName"`
	Dosage      string      `json:"dosage"`
	Quantity    string      `json:"quantity"`
	PerDay      string      `json:"perDay"`
	Dispensed   string      `json:"dispensed"`
	Owed        string      `json:"owed"`
	LastAt      *string     `json:"lastAt"`
	NextDue     *string     `json:"nextDue"`
	Status      string      `json:"status"` // overdue | soon | covered | ""
	Suggest     *SuggestDTO `json:"suggest"`
}

type ScriptDTO struct {
	ID                 int64           `json:"id"`
	PrescriberName     string          `json:"prescriberName"`
	PrescriberReg      string          `json:"prescriberReg"`
	PrescribedOn       string          `json:"prescribedOn"`
	Notes              string          `json:"notes"`
	IsRepeat           bool            `json:"isRepeat"`
	RepeatIntervalDays int             `json:"repeatIntervalDays"`
	Active             bool            `json:"active"`
	Items              []ScriptItemDTO `json:"items"`
}

func iso(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

func ymd(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func parseYMD(s string) (*time.Time, error) {
	if s = strings.TrimSpace(s); s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, apperr.Validation("dates are YYYY-MM-DD")
	}
	return &t, nil
}

func parseQty(s, what string) (decimal.Decimal, error) {
	if s = strings.TrimSpace(s); s == "" {
		return decimal.Zero, nil
	}
	v, err := decimal.NewFromString(s)
	if err != nil {
		return v, apperr.Validation(what + " must be a number")
	}
	return v, nil
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func patientDTO(p Patient) PatientDTO {
	return PatientDTO{ID: p.ID, CustomerID: p.CustomerID, Name: p.Name, Phone: str(p.Phone), DOB: ymd(p.DOB),
		Sex: p.Sex, NIC: p.NIC, Allergies: p.Allergies, Notes: p.Notes}
}

func param(c echo.Context, name string) (int64, error) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.Validation("invalid " + name)
	}
	return id, nil
}

// fail passes app errors through and wraps the rest.
func fail(what string, err error) error {
	if _, ok := apperr.As(err); ok {
		return err
	}
	return apperr.Internal(what, err)
}

type handlers struct{ p *Plugin }

func (h handlers) search(c echo.Context) error {
	rows, err := h.p.store.Search(c.Request().Context(), c.QueryParam("q"), c.QueryParam("customers") == "1", 30)
	if err != nil {
		return fail("search patients", err)
	}
	out := make([]FoundDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, FoundDTO{PatientID: r.PatientID, CustomerID: r.CustomerID, Name: r.Name, Phone: str(r.Phone), NIC: r.NIC, Allergies: r.Allergies})
	}
	return c.JSON(http.StatusOK, out)
}

type profileIn struct {
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	DOB       string `json:"dob"`
	Sex       string `json:"sex"`
	NIC       string `json:"nic"`
	Allergies string `json:"allergies"`
	Notes     string `json:"notes"`
}

func (in profileIn) profile() (Profile, error) {
	dob, err := parseYMD(in.DOB)
	return Profile{DOB: dob, Sex: in.Sex, NIC: in.NIC, Allergies: in.Allergies, Notes: in.Notes}, err
}

func (h handlers) register(c echo.Context) error {
	var in profileIn
	if err := c.Bind(&in); err != nil {
		return apperr.Validation("invalid patient")
	}
	pr, err := in.profile()
	if err != nil {
		return err
	}
	p, err := h.p.store.Register(c.Request().Context(), in.Name, in.Phone, pr)
	if err != nil {
		return fail("register patient", err)
	}
	return c.JSON(http.StatusOK, patientDTO(p))
}

func (h handlers) fromCustomer(c echo.Context) error {
	cid, err := param(c, "cid")
	if err != nil {
		return err
	}
	p, err := h.p.store.FromCustomer(c.Request().Context(), cid)
	if err != nil {
		return fail("make patient", err)
	}
	return c.JSON(http.StatusOK, patientDTO(p))
}

func (h handlers) patient(c echo.Context) error {
	id, err := param(c, "id")
	if err != nil {
		return err
	}
	p, err := h.p.store.Patient(c.Request().Context(), id)
	if err != nil {
		return fail("load patient", err)
	}
	return c.JSON(http.StatusOK, patientDTO(p))
}

func (h handlers) updatePatient(c echo.Context) error {
	id, err := param(c, "id")
	if err != nil {
		return err
	}
	var in profileIn
	if err := c.Bind(&in); err != nil {
		return apperr.Validation("invalid patient")
	}
	pr, err := in.profile()
	if err != nil {
		return err
	}
	if err := h.p.store.UpdateProfile(c.Request().Context(), id, pr); err != nil {
		return fail("save patient", err)
	}
	return h.patient(c)
}

// scripts lists a patient's prescriptions with each drug's due badge, what is
// still owed and what Add would put in the cart.
func (h handlers) scripts(c echo.Context) error {
	id, err := param(c, "id")
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	scs, err := h.p.store.Scripts(ctx, id)
	if err != nil {
		return fail("load prescriptions", err)
	}
	now := time.Now()
	out := make([]ScriptDTO, 0, len(scs))
	for _, sc := range scs {
		d := ScriptDTO{ID: sc.ID, PrescriberName: sc.PrescriberName, PrescriberReg: sc.PrescriberReg, PrescribedOn: ymd(sc.PrescribedOn),
			Notes: sc.Notes, IsRepeat: sc.IsRepeat, RepeatIntervalDays: sc.RepeatIntervalDays, Active: sc.Active, Items: []ScriptItemDTO{}}
		for _, it := range sc.Items {
			due := it.NextDue()
			x := ScriptItemDTO{ID: it.ID, ProductID: it.ProductID, GroupID: it.GroupID, Drug: it.Drug, ProductName: it.ProductName,
				GroupName: it.GroupName, Dosage: it.Dosage, Quantity: it.Quantity.String(), PerDay: it.PerDay.String(),
				Dispensed: it.Dispensed.String(), Owed: it.Owed().String(), LastAt: iso(it.LastAt), NextDue: iso(due)}
			if sc.Active {
				x.Status = dueStatus(due, now)
			}
			if sc.Active && (it.ProductID != nil || it.GroupID != nil) {
				sg, err := h.p.store.Suggest(ctx, it)
				if err != nil {
					return fail("suggest a product", err)
				}
				if sg != nil {
					x.Suggest = &SuggestDTO{ProductID: sg.ProductID, Name: sg.Name, Stock: sg.Stock.String()}
				}
			}
			d.Items = append(d.Items, x)
		}
		out = append(out, d)
	}
	return c.JSON(http.StatusOK, out)
}

type scriptIn struct {
	PatientID          int64  `json:"patientId"`
	PrescriberName     string `json:"prescriberName"`
	PrescriberReg      string `json:"prescriberReg"`
	PrescribedOn       string `json:"prescribedOn"`
	Notes              string `json:"notes"`
	IsRepeat           bool   `json:"isRepeat"`
	RepeatIntervalDays int    `json:"repeatIntervalDays"`
	Items              []struct {
		ID        int64  `json:"id"`
		ProductID *int64 `json:"productId"`
		GroupID   *int64 `json:"groupId"`
		DrugText  string `json:"drugText"`
		Dosage    string `json:"dosage"`
		Quantity  string `json:"quantity"`
		PerDay    string `json:"perDay"`
	} `json:"items"`
}

func bindScript(c echo.Context) (scriptIn, ScriptInput, error) {
	var in scriptIn
	if err := c.Bind(&in); err != nil {
		return in, ScriptInput{}, apperr.Validation("invalid prescription")
	}
	on, err := parseYMD(in.PrescribedOn)
	if err != nil {
		return in, ScriptInput{}, err
	}
	out := ScriptInput{PrescriberName: in.PrescriberName, PrescriberReg: in.PrescriberReg, PrescribedOn: on, Notes: in.Notes,
		IsRepeat: in.IsRepeat, RepeatIntervalDays: in.RepeatIntervalDays}
	for _, it := range in.Items {
		q, err := parseQty(it.Quantity, "quantity")
		if err != nil {
			return in, out, err
		}
		pd, err := parseQty(it.PerDay, "doses a day")
		if err != nil {
			return in, out, err
		}
		out.Items = append(out.Items, ItemInput{ID: it.ID, ProductID: it.ProductID, GroupID: it.GroupID, DrugText: it.DrugText,
			Dosage: it.Dosage, Quantity: q, PerDay: pd})
	}
	return in, out, nil
}

func (h handlers) createScript(c echo.Context) error {
	in, sc, err := bindScript(c)
	if err != nil {
		return err
	}
	id, err := h.p.store.CreateScript(c.Request().Context(), in.PatientID, middleware.CurrentUserID(c), sc)
	if err != nil {
		return fail("save prescription", err)
	}
	return c.JSON(http.StatusOK, map[string]int64{"id": id})
}

func (h handlers) updateScript(c echo.Context) error {
	id, err := param(c, "id")
	if err != nil {
		return err
	}
	_, sc, err := bindScript(c)
	if err != nil {
		return err
	}
	if err := h.p.store.UpdateScript(c.Request().Context(), id, sc); err != nil {
		return fail("save prescription", err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h handlers) scriptActive(c echo.Context) error {
	id, err := param(c, "id")
	if err != nil {
		return err
	}
	var in struct {
		Active bool `json:"active"`
	}
	if err := c.Bind(&in); err != nil {
		return apperr.Validation("invalid request")
	}
	if err := h.p.store.SetScriptActive(c.Request().Context(), id, in.Active); err != nil {
		return fail("stop prescription", err)
	}
	return c.NoContent(http.StatusNoContent)
}
