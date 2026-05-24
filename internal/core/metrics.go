package core

import "time"

type PageMetrics struct {
	PageMS     float64
	TemplateMS float64
}

func pageMetricsSince(start time.Time, templateMS float64) PageMetrics {
	return PageMetrics{
		PageMS:     float64(time.Since(start).Microseconds()) / 1000,
		TemplateMS: templateMS,
	}
}
