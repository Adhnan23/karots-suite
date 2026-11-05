package aicatalog

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"

	"github.com/labstack/echo/v4"
)

const comboSystem = `Suggest deals a shop could offer: products people often buy together. Don't suggest prices.
Never combine prescription medicines. Only use the products listed.
Reply with STRICT JSON ONLY:
{"combos":[{"name":"","product_ids":[],"reason":""}]}`

const comboProductsMax = 200

// ComboIdea is one suggestion; the Combos page opens it in its own form, where the
// person sets the price and saves. Nothing is saved here.
type ComboIdea struct {
	Name       string   `json:"name"`
	ProductIDs []int64  `json:"productIds"`
	Names      []string `json:"names"`
	Reason     string   `json:"reason"`
}

type comboReply struct {
	Ideas []ComboIdea `json:"ideas"`
}

func (h handlers) comboSuggest(c echo.Context) error {
	var in JobRequest
	if err := c.Bind(&in); err != nil {
		return apperr.Validation("invalid request")
	}
	ctx := c.Request().Context()
	s, err := h.settings(c)
	if err != nil {
		return err
	}
	in.Size, in.wide, in.noCap = comboProductsMax, true, false
	prods, _, err := selectProducts(ctx, h.p, "combos", in)
	if err != nil {
		return apperr.Internal("read the products", err)
	}
	prods = prods[:min(len(prods), comboProductsMax)]
	var user strings.Builder
	user.WriteString("PRODUCTS (id|name|category):\n")
	for _, x := range prods {
		user.WriteString(strconv.FormatInt(x.ID, 10) + "|" + x.Name + "|" + x.Category + "\n")
	}
	full := withCustom(comboSystem, s.CustomPrompt) + "\n\n" + user.String() + "\n\n" + jsonOnly
	var out AskReply[comboReply]
	reply := strings.TrimSpace(in.Reply)
	switch {
	case reply != "":
	case s.ManualMode:
		out.Prompt = full
		return c.JSON(http.StatusOK, out)
	default:
		if reply, err = NewClient(s).complete(ctx, comboSystem, user.String(), false); err != nil {
			out.Prompt, out.Error = full, "The AI didn't answer ("+err.Error()+") — copy the prompt to a chatbot instead."
			return c.JSON(http.StatusOK, out)
		}
	}
	ideas, err := h.p.parseCombos(ctx, prods, reply)
	if err != nil {
		out.Prompt, out.Error = full, err.Error()
		return c.JSON(http.StatusOK, out)
	}
	out.Result = &comboReply{Ideas: ideas}
	return c.JSON(http.StatusOK, out)
}

// parseCombos keeps ideas of two or more listed products, none marked as needing a
// prescription by any plugin that keeps that mark.
func (p *Plugin) parseCombos(ctx context.Context, prods []JobProduct, reply string) ([]ComboIdea, error) {
	var r struct {
		Combos []struct {
			Name       string  `json:"name"`
			ProductIDs []int64 `json:"product_ids"`
			Reason     string  `json:"reason"`
		} `json:"combos"`
	}
	if err := decodeReply(reply, &r); err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(prods))
	ids := make([]int64, len(prods))
	for i, x := range prods {
		names[x.ID], ids[i] = x.Name, x.ID
	}
	rx := map[int64]bool{}
	if p.attrSets != nil {
		for _, set := range p.attrSets() {
			vals, err := set.Values(ctx, ids)
			if err != nil {
				return nil, err
			}
			for id, v := range vals {
				if v["rx_only"] == "yes" {
					rx[id] = true
				}
			}
		}
	}
	out := []ComboIdea{}
	for _, c := range r.Combos {
		idea := ComboIdea{Name: strings.TrimSpace(c.Name), Reason: strings.TrimSpace(c.Reason), ProductIDs: []int64{}, Names: []string{}}
		bad := false
		seen := map[int64]bool{}
		for _, id := range c.ProductIDs {
			if rx[id] {
				bad = true
			}
			if names[id] == "" || seen[id] {
				continue
			}
			seen[id] = true
			idea.ProductIDs, idea.Names = append(idea.ProductIDs, id), append(idea.Names, names[id])
		}
		if !bad && idea.Name != "" && len(idea.ProductIDs) >= 2 {
			out = append(out, idea)
		}
	}
	return out, nil
}
