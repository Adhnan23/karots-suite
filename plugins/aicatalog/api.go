package aicatalog

import (
	"net/http"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
)

// The plugin's own JSON, mirrored by hand in web/src/plugins/aicatalog/types.ts.

type SettingsDTO struct {
	ManualMode   bool   `json:"manualMode"`
	Provider     string `json:"provider"`
	BaseURL      string `json:"baseUrl"`
	Model        string `json:"model"`
	APIKey       string `json:"apiKey,omitempty"` // in only: blank = keep; never sent back
	HasKey       bool   `json:"hasKey"`
	WebSearch    bool   `json:"webSearch"`
	NamingStyle  string `json:"namingStyle"`
	CustomPrompt string `json:"customPrompt"`
}

// AskReply is every AI action's answer: the result, or the prompt to copy (paste
// mode, a bad paste, or the API call failed — then Error says why).
type AskReply[T any] struct {
	Result *T     `json:"result,omitempty"`
	Prompt string `json:"prompt,omitempty"`
	Error  string `json:"error,omitempty"`
}

type IdentifyRequest struct {
	Query   string `json:"query"`
	Barcode string `json:"barcode"`
	Hint    string `json:"hint"`
	Reply   string `json:"reply"` // a pasted chatbot answer
	Short   bool   `json:"short"` // the chat already has the rules: send the follow-up prompt
}

type handlers struct{ p *Plugin }

func (p *Plugin) routes(reg *plugin.Registry) {
	h := handlers{p}
	a := reg.Admin() // /api/v1/x — admin + manager
	a.GET("/aicatalog/settings", h.getSettings)
	a.PUT("/aicatalog/settings", h.saveSettings)
	a.POST("/aicatalog/settings/models", h.models)
	a.POST("/aicatalog/settings/test", h.test)
	a.POST("/aicatalog/identify", h.identify)
	a.POST("/aicatalog/category", h.category)
}

func settingsDTO(s Settings) SettingsDTO {
	return SettingsDTO{ManualMode: s.ManualMode, Provider: s.Provider, BaseURL: s.BaseURL, Model: s.Model,
		HasKey: s.APIKey != "", WebSearch: s.WebSearch, NamingStyle: s.NamingStyle, CustomPrompt: s.CustomPrompt}
}

func (h handlers) settings(c echo.Context) (Settings, error) {
	s, err := h.p.store.GetSettings(c.Request().Context())
	if err != nil {
		return s, apperr.Internal("read AI settings", err)
	}
	return s, nil
}

func (h handlers) getSettings(c echo.Context) error {
	s, err := h.settings(c)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, settingsDTO(s))
}

func (h handlers) saveSettings(c echo.Context) error {
	var in SettingsDTO
	if err := c.Bind(&in); err != nil {
		return apperr.Validation("invalid settings")
	}
	if !in.ManualMode && (strings.TrimSpace(in.BaseURL) == "" || strings.TrimSpace(in.Model) == "") {
		return apperr.Validation("pick a provider and a model, or use paste mode")
	}
	s := Settings{ManualMode: in.ManualMode, Provider: strings.TrimSpace(in.Provider), BaseURL: strings.TrimSpace(in.BaseURL),
		Model: strings.TrimSpace(in.Model), APIKey: in.APIKey, WebSearch: in.WebSearch,
		NamingStyle: strings.TrimSpace(in.NamingStyle), CustomPrompt: strings.TrimSpace(in.CustomPrompt)}
	ctx := c.Request().Context()
	if err := h.p.store.SaveSettings(ctx, s); err != nil {
		return apperr.Internal("save AI settings", err)
	}
	if h.p.core.Audit != nil {
		h.p.core.Audit.Record(ctx, middleware.CurrentUserID(c), audit.ActionSettings, "aicatalog", "", "AI Assistant")
	}
	return h.getSettings(c)
}

// typed: the provider fields as typed on the settings page (not yet saved); a
// blank key means the stored one.
func (h handlers) typed(c echo.Context) (Settings, error) {
	var in SettingsDTO
	if err := c.Bind(&in); err != nil {
		return Settings{}, apperr.Validation("invalid settings")
	}
	s, err := h.settings(c)
	if err != nil {
		return s, err
	}
	s.Provider, s.BaseURL, s.Model = in.Provider, in.BaseURL, in.Model
	if k := strings.TrimSpace(in.APIKey); k != "" {
		s.APIKey = k
	}
	return s, nil
}

func (h handlers) models(c echo.Context) error {
	s, err := h.typed(c)
	if err != nil {
		return err
	}
	list, err := NewClient(s).ListModels(c.Request().Context())
	if err != nil {
		return apperr.Validation("Couldn't load models: " + err.Error())
	}
	return c.JSON(http.StatusOK, map[string][]string{"models": list})
}

func (h handlers) test(c echo.Context) error {
	s, err := h.typed(c)
	if err != nil {
		return err
	}
	if err := NewClient(s).Ping(c.Request().Context()); err != nil {
		return apperr.Validation("The AI didn't answer: " + err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

func (h handlers) identify(c echo.Context) error {
	var in IdentifyRequest
	if err := c.Bind(&in); err != nil {
		return apperr.Validation("invalid request")
	}
	if strings.TrimSpace(in.Query) == "" && strings.TrimSpace(in.Barcode) == "" {
		return apperr.Validation("type a name or a code first")
	}
	if in.Query == "" {
		in.Query = in.Barcode
	}
	ctx := c.Request().Context()
	s, err := h.settings(c)
	if err != nil {
		return err
	}
	cats, err := h.p.store.CategoryPaths(ctx)
	if err != nil {
		return apperr.Internal("read categories", err)
	}
	full := func() string { return IdentifyPrompt(in.Query, in.Barcode, in.Hint, cats, s) }
	var out AskReply[IdentifyResult]
	switch {
	case strings.TrimSpace(in.Reply) != "":
		r, perr := parseIdentify(in.Reply)
		if perr == nil {
			out.Result = &r
		} else if out.Error = perr.Error(); in.Short {
			out.Prompt = IdentifyFollowUp(in.Query, in.Barcode, in.Hint)
		} else {
			out.Prompt = full()
		}
	case s.ManualMode && in.Short:
		out.Prompt = IdentifyFollowUp(in.Query, in.Barcode, in.Hint)
	case s.ManualMode:
		out.Prompt = full()
	default:
		r, aerr := NewClient(s).Identify(ctx, in.Query, in.Barcode, in.Hint, cats)
		if aerr == nil {
			out.Result = &r
		} else {
			out.Prompt, out.Error = full(), "The AI didn't answer ("+aerr.Error()+") — copy the prompt to a chatbot instead."
		}
	}
	return c.JSON(http.StatusOK, out)
}

// category finds or creates a picked category path ("A > B").
func (h handlers) category(c echo.Context) error {
	var in struct {
		Path string `json:"path"`
	}
	if err := c.Bind(&in); err != nil {
		return apperr.Validation("invalid request")
	}
	path := normPath(in.Path)
	if path == "" {
		return apperr.Validation("no category given")
	}
	id, err := h.p.cats.FindOrCreateByPath(c.Request().Context(), path)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]int64{"id": id})
}
