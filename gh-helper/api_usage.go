package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

type APIUsageReport struct {
	Backend   string            `json:"backend"`
	Observed  APIUsageObserved  `json:"observed"`
	RateLimit APIUsageRateLimit `json:"rateLimit,omitempty"`
	Warnings  []string          `json:"warnings,omitempty"`
}

type APIUsageObserved struct {
	RESTRequests                   int `json:"restRequests,omitempty"`
	GraphQLRequests                int `json:"graphqlRequests,omitempty"`
	GraphQLCost                    int `json:"graphqlCost,omitempty"`
	GraphQLCostKnownRequests       int `json:"graphqlCostKnownRequests,omitempty"`
	GraphQLCostEstimatedRequests   int `json:"graphqlCostEstimatedRequests,omitempty"`
	GraphQLCostUnavailableRequests int `json:"graphqlCostUnavailableRequests,omitempty"`
	GraphQLNodeCount               int `json:"graphqlNodeCount,omitempty"`
}

type APIUsageRateLimit struct {
	Core    APIUsageRateLimitResource `json:"core,omitempty"`
	GraphQL APIUsageRateLimitResource `json:"graphql,omitempty"`
}

type APIUsageRateLimitResource struct {
	Remaining      *int   `json:"remaining,omitempty"`
	Used           *int   `json:"used,omitempty"`
	Limit          *int   `json:"limit,omitempty"`
	ResetAt        string `json:"resetAt,omitempty"`
	ResetInSeconds *int64 `json:"resetInSeconds,omitempty"`
}

type rateLimitSnapshot struct {
	Core    rateLimitResource
	GraphQL rateLimitResource
}

type rateLimitResource struct {
	Seen      bool
	Limit     int
	Remaining int
	Used      int
	Reset     int64
}

type apiUsageRecorder struct {
	mu       sync.Mutex
	enabled  bool
	finished bool
	report   *APIUsageReport

	observed        APIUsageObserved
	after           *rateLimitSnapshot
	warnings        []string
	lastGraphQLUsed *int

	warningThreshold float64
	warningWritten   bool
	structuredOutput bool
}

var commandAPIUsage = &apiUsageRecorder{}

func startAPIUsageForCommand(_ *cobra.Command, _ []string) {
	if !apiUsage {
		commandAPIUsage.Reset(false)
		commandAPIUsage.SetWarningThreshold(rateLimitWarningThreshold)
		return
	}

	commandAPIUsage.Reset(true)
	commandAPIUsage.SetWarningThreshold(rateLimitWarningThreshold)
}

func printAPIUsageForTextCommand(cmd *cobra.Command, _ []string) {
	if !apiUsage || commandAPIUsage.StructuredOutputWritten() {
		return
	}
	report := commandAPIUsage.Finish()
	if _, err := fmt.Fprintln(cmd.ErrOrStderr(), formatAPIUsageSummary(report)); err != nil {
		slog.Warn("failed to write api usage summary", "error", err)
	}
}

func apiUsageEnabledForCommand(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	enabled, err := cmd.Root().PersistentFlags().GetBool("api-usage")
	return err == nil && enabled
}

func attachAPIUsageToOutput(cmd *cobra.Command, data interface{}) interface{} {
	if !apiUsageEnabledForCommand(cmd) {
		return data
	}

	report := commandAPIUsage.Finish()
	commandAPIUsage.MarkStructuredOutputWritten()

	if dataMap, ok := data.(map[string]interface{}); ok {
		dataMap["apiUsage"] = report
		return dataMap
	}

	return map[string]interface{}{
		"data":     data,
		"apiUsage": report,
	}
}

func formatAPIUsageSummary(report *APIUsageReport) string {
	if report == nil {
		return "apiUsage: unavailable"
	}
	parts := []string{
		fmt.Sprintf("backend=%s", report.Backend),
		fmt.Sprintf("restRequests=%d", report.Observed.RESTRequests),
		fmt.Sprintf("graphqlRequests=%d", report.Observed.GraphQLRequests),
	}
	if report.Observed.GraphQLCost > 0 {
		parts = append(parts, fmt.Sprintf("graphqlCost=%d", report.Observed.GraphQLCost))
	}
	if report.RateLimit.Core.Remaining != nil {
		parts = append(parts, fmt.Sprintf("coreRemaining=%d", *report.RateLimit.Core.Remaining))
	}
	if report.RateLimit.GraphQL.Remaining != nil {
		parts = append(parts, fmt.Sprintf("graphqlRemaining=%d", *report.RateLimit.GraphQL.Remaining))
	}
	if len(report.Warnings) > 0 {
		parts = append(parts, fmt.Sprintf("warnings=%d", len(report.Warnings)))
	}
	return "apiUsage: " + strings.Join(parts, " ")
}

func (r *apiUsageRecorder) Reset(enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.enabled = enabled
	r.finished = false
	r.report = nil
	r.observed = APIUsageObserved{}
	r.after = nil
	r.warnings = nil
	r.lastGraphQLUsed = nil
	r.warningThreshold = 0
	r.warningWritten = false
	r.structuredOutput = false
}

func (r *apiUsageRecorder) SetWarningThreshold(threshold float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.warningThreshold = threshold
}

func (r *apiUsageRecorder) RecordGraphQLResponse(headers http.Header, body []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.shouldTrackLocked() {
		return
	}
	r.observed.GraphQLRequests++
	previousUsed := r.lastGraphQLUsed
	r.updateAfterFromHeaders("graphql", headers)
	if rateLimit, ok := graphQLRateLimitFromResponse(body); ok {
		r.observed.GraphQLCost += rateLimit.Cost
		r.observed.GraphQLCostKnownRequests++
		r.observed.GraphQLNodeCount += rateLimit.NodeCount
		r.updateAfterFromGraphQLRateLimit(rateLimit)
		r.lastGraphQLUsed = apiUsageIntPtr(rateLimit.Used)
		return
	}
	if used, ok := parseHeaderInt(headers, "x-ratelimit-used"); ok {
		if previousUsed != nil && used > *previousUsed {
			r.observed.GraphQLCost += used - *previousUsed
			r.observed.GraphQLCostEstimatedRequests++
		} else {
			r.observed.GraphQLCostUnavailableRequests++
		}
		r.lastGraphQLUsed = apiUsageIntPtr(used)
		return
	}
	r.observed.GraphQLCostUnavailableRequests++
}

func (r *apiUsageRecorder) RecordRESTResponse(headers http.Header) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.shouldTrackLocked() {
		return
	}
	r.observed.RESTRequests++
	r.updateAfterFromHeaders("core", headers)
}

func (r *apiUsageRecorder) AddWarning(warning string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.enabled {
		return
	}
	r.warnings = append(r.warnings, warning)
}

func (r *apiUsageRecorder) StructuredOutputWritten() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.structuredOutput
}

func (r *apiUsageRecorder) MarkStructuredOutputWritten() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.enabled {
		r.structuredOutput = true
	}
}

func (r *apiUsageRecorder) shouldTrackLocked() bool {
	return r.enabled || r.warningThreshold > 0
}

func (r *apiUsageRecorder) Finish() *APIUsageReport {
	r.mu.Lock()
	if r.finished {
		report := r.report
		r.mu.Unlock()
		return report
	}
	defer r.mu.Unlock()

	report := r.buildReportLocked()
	r.report = report
	r.finished = true
	return report
}

func (r *apiUsageRecorder) buildReportLocked() *APIUsageReport {
	observed := r.observed

	return &APIUsageReport{
		Backend:   observedBackend(observed),
		Observed:  observed,
		RateLimit: buildRateLimitReport(r.after),
		Warnings:  append([]string(nil), r.warnings...),
	}
}

func (r *apiUsageRecorder) updateAfterFromHeaders(resource string, headers http.Header) {
	snapshot := r.after
	if snapshot == nil {
		snapshot = &rateLimitSnapshot{}
		r.after = snapshot
	}

	limit, limitOK := parseHeaderInt(headers, "x-ratelimit-limit")
	remaining, remainingOK := parseHeaderInt(headers, "x-ratelimit-remaining")
	used, usedOK := parseHeaderInt(headers, "x-ratelimit-used")
	reset, resetOK := parseHeaderInt64(headers, "x-ratelimit-reset")

	target := &snapshot.Core
	if resource == "graphql" {
		target = &snapshot.GraphQL
	}
	target.Seen = target.Seen || limitOK || remainingOK || usedOK || resetOK
	if limitOK {
		target.Limit = limit
	}
	if remainingOK {
		target.Remaining = remaining
	}
	if usedOK {
		target.Used = used
	}
	if resetOK {
		target.Reset = reset
	}
	r.maybeWarnRateLimitLocked(resource, *target)
}

func (r *apiUsageRecorder) updateAfterFromGraphQLRateLimit(rateLimit graphQLRateLimitTelemetry) {
	snapshot := r.after
	if snapshot == nil {
		snapshot = &rateLimitSnapshot{}
		r.after = snapshot
	}
	target := &snapshot.GraphQL
	target.Seen = true
	if rateLimit.Limit != 0 {
		target.Limit = rateLimit.Limit
	}
	target.Remaining = rateLimit.Remaining
	target.Used = rateLimit.Used
	if rateLimit.ResetAt != "" {
		if resetAt, err := time.Parse(time.RFC3339, rateLimit.ResetAt); err == nil {
			target.Reset = resetAt.Unix()
		}
	}
	r.maybeWarnRateLimitLocked("graphql", *target)
}

func (r *apiUsageRecorder) maybeWarnRateLimitLocked(resource string, rateLimit rateLimitResource) {
	if r.warningWritten || r.warningThreshold <= 0 || rateLimit.Limit <= 0 {
		return
	}
	usedRatio := float64(rateLimit.Used) / float64(rateLimit.Limit)
	if usedRatio < r.warningThreshold {
		return
	}

	resetAt := "unknown"
	resetIn := ""
	if rateLimit.Reset != 0 {
		resetAtTime := time.Unix(rateLimit.Reset, 0).UTC()
		resetAt = resetAtTime.Format(time.RFC3339)
		resetIn = fmt.Sprintf(" in %s", formatResetIn(time.Until(resetAtTime)))
	}
	_, _ = fmt.Fprintf(
		os.Stderr,
		"warning: GitHub %s API rate limit is %.0f%% used (%d/%d used, %d remaining, resets at %s%s)\n",
		resource,
		usedRatio*100,
		rateLimit.Used,
		rateLimit.Limit,
		rateLimit.Remaining,
		resetAt,
		resetIn,
	)
	r.warningWritten = true
}

func observedBackend(observed APIUsageObserved) string {
	switch {
	case observed.RESTRequests > 0 && observed.GraphQLRequests > 0:
		return "mixed"
	case observed.RESTRequests > 0:
		return "rest"
	case observed.GraphQLRequests > 0:
		return "graphql"
	default:
		return "none"
	}
}

func buildRateLimitReport(after *rateLimitSnapshot) APIUsageRateLimit {
	return APIUsageRateLimit{
		Core:    buildRateLimitResourceReport(resourceFromSnapshot(after, "core")),
		GraphQL: buildRateLimitResourceReport(resourceFromSnapshot(after, "graphql")),
	}
}

func resourceFromSnapshot(snapshot *rateLimitSnapshot, resource string) *rateLimitResource {
	if snapshot == nil {
		return nil
	}
	if resource == "graphql" {
		return &snapshot.GraphQL
	}
	return &snapshot.Core
}

func buildRateLimitResourceReport(after *rateLimitResource) APIUsageRateLimitResource {
	report := APIUsageRateLimitResource{}
	if after == nil || !after.Seen {
		return report
	}

	report.Remaining = apiUsageIntPtr(after.Remaining)
	report.Used = apiUsageIntPtr(after.Used)
	if after.Limit != 0 {
		report.Limit = apiUsageIntPtr(after.Limit)
	}
	if after.Reset != 0 {
		resetAt := time.Unix(after.Reset, 0).UTC()
		report.ResetAt = resetAt.Format(time.RFC3339)
		report.ResetInSeconds = apiUsageInt64Ptr(maxDurationSeconds(time.Until(resetAt)))
	}
	return report
}

func apiUsageIntPtr(value int) *int {
	return &value
}

func apiUsageInt64Ptr(value int64) *int64 {
	return &value
}

func maxDurationSeconds(duration time.Duration) int64 {
	seconds := int64(duration.Round(time.Second).Seconds())
	if seconds < 0 {
		return 0
	}
	return seconds
}

func formatResetIn(duration time.Duration) string {
	if duration <= 0 {
		return "now"
	}
	duration = duration.Round(time.Second)
	if duration < time.Minute {
		return duration.String()
	}
	duration = duration.Round(time.Minute)
	if duration < time.Hour {
		return duration.String()
	}
	duration = duration.Round(time.Minute)
	return duration.String()
}

func parseHeaderInt(headers http.Header, name string) (int, bool) {
	value := headers.Get(name)
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func parseHeaderInt64(headers http.Header, name string) (int64, bool) {
	value := headers.Get(name)
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}
