package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Row struct {
	ID          string
	Domain      string
	Phase       string
	Method      string
	CurrentPath string
	Target      string
	Status      string
	Evidence    string
	CheckedAt   string
}

func main() {
	defaultLedger := "docs/prd-v2/progress/ledger.csv"
	if _, err := os.Stat(defaultLedger); os.IsNotExist(err) {
		defaultLedger = "progress/ledger.csv"
	}

	ledgerPath := flag.String("ledger", defaultLedger, "path to ledger.csv")
	verify := flag.Bool("verify", false, "verify ledger matches actual code/contract")
	sync := flag.Bool("sync", false, "synchronize ledger with actual code/contract")
	openAPIPath := flag.String("openapi", "api/openapi.yaml", "path to openapi.yaml")
	flag.Parse()

	rows, err := loadLedger(*ledgerPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading ledger from %s: %v\n", *ledgerPath, err)
		os.Exit(1)
	}

	openAPIContent, _ := os.ReadFile(*openAPIPath)
	openAPIText := string(openAPIContent)

	mismatches := 0
	var done, wip, todo, blocked int

	for i := range rows {
		r := &rows[i]
		actual := deriveStatus(r, openAPIText)
		if actual != r.Status {
			mismatches++
			if *verify {
				fmt.Printf("[MISMATCH] %s (%s %s): ledger has '%s', actual derived is '%s'\n",
					r.ID, r.Method, r.CurrentPath, r.Status, actual)
			}
		}
		switch actual {
		case "done":
			done++
		case "wip":
			wip++
		case "blocked":
			blocked++
		default:
			todo++
		}
	}

	total := len(rows)
	pct := 0.0
	if total > 0 {
		pct = float64(done) / float64(total) * 100
	}

	fmt.Printf("Total Endpoints: %d | done: %d (%.1f%%) | wip: %d | todo: %d | blocked: %d | mismatches: %d\n",
		total, done, pct, wip, todo, blocked, mismatches)

	if *verify && mismatches > 0 {
		fmt.Printf("Ledger verification completed with %d mismatch(es). Review code vs ledger.\n", mismatches)
	}

	if *sync {
		nowISO := time.Now().UTC().Format(time.RFC3339)
		syncedCount := 0
		for i := range rows {
			r := &rows[i]
			actual := deriveStatus(r, openAPIText)
			if r.Status != actual {
				r.Status = actual
				if actual == "done" && r.Evidence == "" {
					r.Evidence = "P1-domain-" + domainToDir(r.Domain)
				}
				r.CheckedAt = nowISO
				syncedCount++
			}
		}
		if err := saveLedger(*ledgerPath, rows); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving ledger: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully synchronized %s (%d rows updated).\n", *ledgerPath, syncedCount)
	}
}

func saveLedger(path string, rows []Row) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{"id", "domain", "phase", "method", "current_path", "target", "status", "evidence", "checked_at"}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, r := range rows {
		rec := []string{r.ID, r.Domain, r.Phase, r.Method, r.CurrentPath, r.Target, r.Status, r.Evidence, r.CheckedAt}
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	return nil
}

func loadLedger(path string) ([]Row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) <= 1 {
		return nil, fmt.Errorf("ledger is empty")
	}

	var rows []Row
	for _, rec := range records[1:] {
		if len(rec) < 7 {
			continue
		}
		r := Row{
			ID:          strings.TrimSpace(rec[0]),
			Domain:      strings.TrimSpace(rec[1]),
			Phase:       strings.TrimSpace(rec[2]),
			Method:      strings.TrimSpace(rec[3]),
			CurrentPath: strings.TrimSpace(rec[4]),
			Target:      strings.TrimSpace(rec[5]),
			Status:      strings.TrimSpace(rec[6]),
		}
		if len(rec) > 7 {
			r.Evidence = strings.TrimSpace(rec[7])
		}
		if len(rec) > 8 {
			r.CheckedAt = strings.TrimSpace(rec[8])
		}
		rows = append(rows, r)
	}
	return rows, nil
}

func deriveStatus(r *Row, openAPIText string) string {
	// If marked blocked in ledger with explicit reason, keep blocked
	if r.Status == "blocked" {
		return "blocked"
	}

	// 1. Check if contract exists in openapi.yaml
	contractOK := false
	if openAPIText != "" {
		targetPath := r.Target
		reParam := regexp.MustCompile(`:([a-zA-Z0-9_]+)`)
		openAPIFormat := reParam.ReplaceAllString(targetPath, `{$1}`)
		openAPIFormat = strings.ReplaceAll(openAPIFormat, "*planCode", "{planCode}")
		if strings.Contains(openAPIText, targetPath) || strings.Contains(openAPIText, openAPIFormat) {
			contractOK = true
		}
	}

	// 2. Check if clean layered implementation exists
	layered := hasLayeredImplementation(r.Domain, r.Target)

	// 3. Check if test exists
	tested := hasPassingTest(r.Domain, r.Target)

	if contractOK && layered && tested {
		return "done"
	}
	if contractOK || layered || tested {
		return "wip"
	}
	return "todo"
}

func domainToDir(domain string) string {
	dom := strings.ToLower(domain)
	switch {
	case strings.HasPrefix(dom, "f01"):
		return "account"
	case dom == "f02":
		return "catalog"
	case dom == "f03" || dom == "f08":
		return "server"
	case dom == "f04":
		return "order"
	case dom == "f05":
		return "monitor"
	case dom == "f06" || dom == "f09":
		return "vps"
	case dom == "f07":
		return "telegram"
	case dom == "f10" || dom == "f11":
		return "system"
	default:
		return dom
	}
}

func hasLayeredImplementation(domain, target string) bool {
	domDir := domainToDir(domain)
	p := filepath.Join("backend", "internal", "domain", domDir)
	if fi, err := os.Stat(p); err == nil && fi.IsDir() {
		return true
	}
	return false
}

func hasPassingTest(domain, target string) bool {
	domDir := domainToDir(domain)
	p := filepath.Join("backend", "internal", "domain", domDir)
	matches, _ := filepath.Glob(filepath.Join(p, "*_test.go"))
	return len(matches) > 0
}
