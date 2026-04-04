package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
)

// writeUsageCSV writes UsageResult rows to a CSV file.
// Columns: file,account_id,email,plan_type,5h_used_pct,5h_reset_at,weekly_used_pct,weekly_reset_at,upstream_status
func writeUsageCSV(path string, rows []UsageResult) error {
	f, err := openCSVFile(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{
		"file", "account_id", "email", "plan_type",
		"5h_used_pct", "5h_reset_at",
		"weekly_used_pct", "weekly_reset_at",
		"upstream_status", "error",
	}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, r := range rows {
		errStr := ""
		if r.Err != nil {
			errStr = r.Err.Error()
		}
		row := []string{
			r.File,
			r.AccountID,
			r.Email,
			r.PlanType,
			windowPct(r.FiveHour),
			windowResetAt(r.FiveHour),
			windowPct(r.Weekly),
			windowResetAt(r.Weekly),
			strconv.Itoa(r.UpstreamStatus),
			errStr,
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return w.Error()
}

// writeAPITestCSV writes one row per credential; available = at least one of 3 random sample models succeeded.
// Columns: file,account_id,sample_models,available
func writeAPITestCSV(path string, rows []APITestTokenSummary) error {
	f, err := openCSVFile(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{"file", "account_id", "sample_models", "available"}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, r := range rows {
		row := []string{
			r.File,
			r.AccountID,
			r.SampleModels,
			boolStr(r.Available),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return w.Error()
}

func openCSVFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("create CSV file %s: %w", path, err)
	}
	return f, nil
}

func windowPct(w *WindowInfo) string {
	if w == nil {
		return ""
	}
	return fmt.Sprintf("%.1f", w.UsedPercent)
}

func windowResetAt(w *WindowInfo) string {
	if w == nil {
		return ""
	}
	return formatUnixTS(w.ResetAt)
}

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
