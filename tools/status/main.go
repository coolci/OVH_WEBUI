package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type RegisteredRoute struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Handler string `json:"handler"`
}

var registeredRoutes = map[string]bool{}
var testResults = map[string]bool{}

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

	openAPIContent, err := os.ReadFile(*openAPIPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading OpenAPI contract from %s: %v\n", *openAPIPath, err)
		os.Exit(1)
	}
	openAPIText := string(openAPIContent)
	var parsedContract map[string]interface{}
	if err := yaml.Unmarshal(openAPIContent, &parsedContract); err != nil {
		fmt.Fprintln(os.Stderr, "Invalid OpenAPI contract:", err)
		os.Exit(1)
	}
	if err := loadRoutes(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	covered := map[string]bool{}
	invalid := 0
	for _, r := range rows {
		key := routeKey(r.Method, r.CurrentPath)
		if !registeredRoutes[key] || covered[key] {
			fmt.Fprintf(os.Stderr, "Invalid or duplicate registry entry: %s %s\n", r.Method, r.CurrentPath)
			invalid++
		}
		covered[key] = true
	}
	for key := range registeredRoutes {
		if !covered[key] {
			fmt.Fprintln(os.Stderr, "Untracked route:", key)
			invalid++
		}
	}
	if invalid > 0 && (*verify || *sync) {
		os.Exit(1)
	}

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
		fmt.Fprintf(os.Stderr, "Ledger verification failed with %d mismatch(es). Review code vs ledger.\n", mismatches)
		os.Exit(1)
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
					r.Evidence = ""
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
	w.Flush()
	return w.Error()
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

func routeKey(method, path string) string {
	// Parameter names do not change the route contract; wildcard and single
	// segment parameters remain distinct.
	path = regexp.MustCompile(`:[A-Za-z0-9_]+|\{[^}]+\}`).ReplaceAllString(path, ":param")
	path = regexp.MustCompile(`\*[A-Za-z0-9_]+`).ReplaceAllString(path, "*param")
	return strings.ToUpper(method) + " " + path
}
func loadRoutes() error {
	cmd := exec.Command("go", "run", "./backend/cmd/routes")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("export registry: %w: %s", err, stderr.String())
	}
	var routes []RegisteredRoute
	if err = json.Unmarshal(data, &routes); err != nil {
		return err
	}
	if len(routes) == 0 {
		return fmt.Errorf("empty route registry")
	}
	registeredRoutes = map[string]bool{}
	for _, r := range routes {
		if r.Handler == "" {
			return fmt.Errorf("route without handler")
		}
		registeredRoutes[routeKey(r.Method, r.Path)] = true
	}
	return nil
}
func contractExists(r *Row, content string) bool {
	var contract struct {
		Paths map[string]map[string]interface{} `yaml:"paths"`
	}
	if yaml.Unmarshal([]byte(content), &contract) != nil {
		return false
	}
	for path, methods := range contract.Paths {
		target := strings.ReplaceAll(r.Target, "*planCode", ":planCode")
		if routeKey(r.Method, path) == routeKey(r.Method, target) {
			_, ok := methods[strings.ToLower(r.Method)]
			return ok
		}
	}
	return false
}
func deriveStatus(r *Row, content string) string {
	implemented := registeredRoutes[routeKey(r.Method, r.CurrentPath)]
	if !implemented {
		return "todo"
	}
	if r.Status == "blocked" {
		return "blocked"
	}
	if contractExists(r, content) && verifiedTest(r.Evidence) {
		return "done"
	}
	return "wip"
}

// Evidence explicitly associates this ledger row with one runnable named Go
// regression. A directory or a test file alone cannot establish a passing test.
func verifiedTest(evidence string) bool {
	const prefix = "go-test:"
	if !strings.HasPrefix(evidence, prefix) {
		return false
	}
	if result, ok := testResults[evidence]; ok {
		return result
	}
	parts := strings.Split(strings.TrimPrefix(evidence, prefix), "#")
	if len(parts) != 2 || !regexp.MustCompile(`^\./backend/internal/[a-zA-Z0-9_/-]+$`).MatchString(parts[0]) || !regexp.MustCompile(`^Test[A-Za-z0-9_]+$`).MatchString(parts[1]) {
		return false
	}
	cmd := exec.Command("go", "test", parts[0], "-run", "^"+parts[1]+"$", "-count=1", "-json")
	data, err := cmd.Output()
	passed := false
	if err == nil {
		for _, line := range bytes.Split(data, []byte("\n")) {
			var event struct{ Action, Test string }
			if json.Unmarshal(line, &event) == nil && event.Action == "pass" && event.Test == parts[1] {
				passed = true
			}
		}
	}
	testResults[evidence] = passed
	return passed
}
