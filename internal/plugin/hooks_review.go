package plugin

import "context"

// ReviewSource lets a plugin put its own products on core's Items to review (REV-1), each with a short note
// ("From the catalogue"), and clear them when the product is marked reviewed (REV-2). Notes is called for the
// list and its badge: keep it one query. Key is sent with each note so the web half can add its own buttons
// to that row (the reviewActions slot).
type ReviewSource struct {
	Key   string
	Notes func(ctx context.Context) (map[int64][]string, error)
	Clear func(ctx context.Context, productID int64) error
}

var reviewSources hook[ReviewSource]

func (r *Registry) AddReviewSource(s ReviewSource) { reviewSources.add(s) }

// ReviewSources are the registered review sources, in registration order.
func ReviewSources() []ReviewSource { return reviewSources.all() }
