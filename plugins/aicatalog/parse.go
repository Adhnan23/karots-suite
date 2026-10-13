package aicatalog

import (
	"encoding/json"
	"errors"
	"strings"
)

type Candidate struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Specs       string `json:"specs"`
	Explanation string `json:"explanation"`
}

type IdentifyResult struct {
	Confident bool        `json:"confident"`
	Best      Candidate   `json:"best"`
	Options   []Candidate `json:"options"`
}

// CleanupPlan is one clean-up answer: karots-pos's category plan plus
// uncategorized placements, product renames and look-alike pairs.
type CleanupPlan struct {
	Renames []struct {
		ID     int64  `json:"id"`
		To     string `json:"to"`
		Reason string `json:"reason"`
	} `json:"renames"`
	Reparents []struct {
		ID       int64  `json:"id"`
		ParentID int64  `json:"parent_id"` // 0 = make top-level
		Reason   string `json:"reason"`
	} `json:"reparents"`
	Merges []struct {
		From   int64  `json:"from"`
		Into   int64  `json:"into"`
		Reason string `json:"reason"`
	} `json:"merges"`
	Moves []struct {
		ProductID  int64  `json:"product_id"`
		CategoryID int64  `json:"category_id"`
		Reason     string `json:"reason"`
	} `json:"moves"`
	Uncategorized []struct {
		ProductID int64  `json:"product_id"`
		Category  string `json:"category"`
		Reason    string `json:"reason"`
	} `json:"uncategorized"`
	Names []struct {
		ProductID int64  `json:"product_id"`
		To        string `json:"to"`
		Reason    string `json:"reason"`
	} `json:"names"`
	Duplicates []struct {
		A      int64  `json:"a"`
		B      int64  `json:"b"`
		Reason string `json:"reason"`
	} `json:"duplicates"`
}

// errNoAnswer is what the reply box shows for a paste with no usable JSON.
var errNoAnswer = errors.New("Couldn't find an answer in that — paste the whole reply (it should contain JSON starting with {).")

// jsonObject cuts the outermost {…} out of a chatbot reply, so prose and code
// fences around it don't matter.
func jsonObject(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	i := strings.IndexByte(s, '{')
	j := strings.LastIndexByte(s, '}')
	if i < 0 || j < i {
		return "", errNoAnswer
	}
	return s[i : j+1], nil
}

func parseIdentify(raw string) (IdentifyResult, error) {
	var out IdentifyResult
	s, err := jsonObject(raw)
	if err != nil {
		return out, err
	}
	if json.Unmarshal([]byte(s), &out) != nil {
		return IdentifyResult{}, errNoAnswer
	}
	out.Best.Category = normPath(out.Best.Category)
	var opts []Candidate
	for _, o := range out.Options {
		if strings.TrimSpace(o.Name) != "" {
			o.Category = normPath(o.Category)
			opts = append(opts, o)
		}
	}
	if len(opts) > 4 {
		opts = opts[:4]
	}
	out.Options = opts
	if out.Confident && strings.TrimSpace(out.Best.Name) == "" {
		out.Confident = false
	}
	if !out.Confident && len(out.Options) == 0 {
		if strings.TrimSpace(out.Best.Name) == "" {
			return IdentifyResult{}, errors.New("That reply has no product in it — ask the chatbot again, or explain the item.")
		}
		out.Options = []Candidate{out.Best}
	}
	return out, nil
}

func parseCleanup(raw string) (CleanupPlan, error) {
	var out CleanupPlan
	s, err := jsonObject(raw)
	if err != nil {
		return out, err
	}
	if json.Unmarshal([]byte(s), &out) != nil {
		return CleanupPlan{}, errNoAnswer
	}
	for i := range out.Uncategorized {
		out.Uncategorized[i].Category = normPath(out.Uncategorized[i].Category)
	}
	return out, nil
}

// normPath tidies an AI category path to core's "A > B" form. Only a spaced
// " / " counts as a level break — "1/2 inch" stays one name.
func normPath(p string) string {
	parts := strings.Split(strings.ReplaceAll(p, " / ", ">"), ">")
	out := parts[:0]
	for _, x := range parts {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return strings.Join(out, " > ")
}
