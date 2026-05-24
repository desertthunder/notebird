package core

import "time"

// PageMetrics contains lightweight timings shown in the site footer.
// Full-page handlers that render the base template should always set this field
// on PageData so every page reports comparable footer metrics.
type PageMetrics struct {
	PageMS     float64
	TemplateMS float64
}

// pageMetricsSince returns elapsed request preparation time plus the most recent
// template render duration. Call it immediately before execute for full-page
// responses; execute updates TemplateMS after the response is rendered, so the
// displayed template value is intentionally the previous render's duration.
func pageMetricsSince(start time.Time, templateMS float64) PageMetrics {
	return PageMetrics{
		PageMS:     float64(time.Since(start).Microseconds()) / 1000,
		TemplateMS: templateMS,
	}
}
