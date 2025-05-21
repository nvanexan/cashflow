package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Transaction struct {
	Date            time.Time
	Type            string
	Amount          float64
	Description     string
	Tags            []string
	ProjectedAmount *float64 // nil if not specified
}

// CLI flags
var (
	filterTag      string
	filterType     string
	fromDate       string
	toDate         string
	removeTags     string
	adjustTags     string
	exportMarkdown string
	file           string
)

func init() {
	flag.StringVar(&filterTag, "tag", "", "Filter transactions by tag")
	flag.StringVar(&filterType, "type", "", "Filter by type: income or expense")
	flag.StringVar(&fromDate, "from", "", "Start date YYYY-MM-DD")
	flag.StringVar(&toDate, "to", "", "End date YYYY-MM-DD")
	flag.StringVar(&removeTags, "remove", "", "Comma-separated tags to remove")
	flag.StringVar(&adjustTags, "adjust", "", "Tag adjustments e.g. Food=-0.5,Salary=0.1")
	flag.StringVar(&exportMarkdown, "export-md", "", "Export side-by-side projection as a Markdown file")
	flag.StringVar(&file, "file", "sample-cashflow.md", "Cashflow markdown file to process")
}

func main() {
	flag.Parse()

	transactions, err := parseSimpleMarkdown(file)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	transactions = applyFilters(transactions)
	printSummary(transactions)

	projection := buildProjection(transactions, adjustTags)
	printSideBySide(projection)

	if exportMarkdown != "" {
		err := exportProjectionMarkdown(projection, exportMarkdown)
		if err != nil {
			fmt.Println("Error writing markdown:", err)
		} else {
			fmt.Println("📁 Exported projection to:", exportMarkdown)
		}
	}

}

func parseSimpleMarkdown(filename string) ([]Transaction, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var transactions []Transaction
	var currentDate time.Time

	scanner := bufio.NewScanner(file)

	dateRegex := regexp.MustCompile(`^#\s+(\d{4}-\d{2}-\d{2})$`)
	// Matches: - 9.49 Coffee [Tag1, Tag2] (5.20)
	txnRegex := regexp.MustCompile(`^([+-])\s*([\d.]+)\s+(.+?)(?:\s+\[([^\]]+)\])?(?:\s+\(([\d.]+)\))?$`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if matches := dateRegex.FindStringSubmatch(line); len(matches) == 2 {
			date, err := time.Parse("2006-01-02", matches[1])
			if err == nil {
				currentDate = date
			}
			continue
		}

		if matches := txnRegex.FindStringSubmatch(line); len(matches) >= 3 {
			sign := matches[1]
			amount, err := strconv.ParseFloat(matches[2], 64)
			if err != nil {
				continue
			}
			if sign == "-" {
				amount = -amount
			}

			description := strings.TrimSpace(matches[3])
			tags := []string{}
			if len(matches) >= 5 && matches[4] != "" {
				tags = strings.Split(matches[4], ",")
				for i := range tags {
					tags[i] = strings.TrimSpace(tags[i])
				}
			}

			var projectedAmount *float64
			if len(matches) >= 6 && matches[5] != "" {
				p, err := strconv.ParseFloat(matches[5], 64)
				if err == nil {
					projectedAmount = &p
				}
			}

			transactions = append(transactions, Transaction{
				Date:            currentDate,
				Type:            map[bool]string{true: "income", false: "expense"}[amount >= 0],
				Amount:          amount,
				Description:     description,
				Tags:            tags,
				ProjectedAmount: projectedAmount,
			})
		}

	}

	return transactions, scanner.Err()
}

func applyFilters(transactions []Transaction) []Transaction {
	var result []Transaction

	var from, to time.Time
	var err error
	if fromDate != "" {
		from, err = time.Parse("2006-01-02", fromDate)
		if err != nil {
			fmt.Println("Invalid --from date format")
			os.Exit(1)
		}
	}
	if toDate != "" {
		to, err = time.Parse("2006-01-02", toDate)
		if err != nil {
			fmt.Println("Invalid --to date format")
			os.Exit(1)
		}
	}

	// ✅ Parse remove tags once
	removeSet := parseRemovals(removeTags)

	for _, txn := range transactions {
		// ✅ Skip if any tag matches remove set
		if hasAnyTag(txn, removeSet) {
			continue
		}
		if filterTag != "" && !hasTag(txn, filterTag) {
			continue
		}
		if filterType != "" && !strings.EqualFold(txn.Type, filterType) {
			continue
		}
		if !from.IsZero() && txn.Date.Before(from) {
			continue
		}
		if !to.IsZero() && txn.Date.After(to) {
			continue
		}
		result = append(result, txn)
	}

	return result
}

func hasTag(txn Transaction, tag string) bool {
	for _, t := range txn.Tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

func printSummary(transactions []Transaction) {
	fmt.Println("📊 Filtered Cash Flow Summary:")
	var incomeTotal, expenseTotal float64
	for _, txn := range transactions {
		fmt.Printf("%s [%s] %.2f - %s %v\n",
			txn.Date.Format("2006-01-02"),
			txn.Type,
			txn.Amount,
			txn.Description,
			txn.Tags,
		)
		if txn.Amount >= 0 {
			incomeTotal += txn.Amount
		} else {
			expenseTotal += txn.Amount
		}
	}

	fmt.Printf("\nTotal Income:  %.2f\n", incomeTotal)
	fmt.Printf("Total Expenses: %.2f\n", -expenseTotal)
	fmt.Printf("Net:            %.2f\n\n", incomeTotal+expenseTotal)

	printTagSummary(transactions)
	fmt.Println()

	printHighImpactTags(transactions, 10) // Top 10 by default

	fmt.Println()
}

func printTagSummary(transactions []Transaction) {
	tagSums := make(map[string]float64)

	for _, txn := range transactions {
		if len(txn.Tags) == 0 {
			tagSums["_untagged_"] += txn.Amount
		} else {
			for _, tag := range txn.Tags {
				tagSums[tag] += txn.Amount
			}
		}
	}

	fmt.Println("📌 Totals by Tag:")
	keys := make([]string, 0, len(tagSums))
	for tag := range tagSums {
		keys = append(keys, tag)
	}
	sort.Strings(keys)

	for _, tag := range keys {
		total := tagSums[tag]
		category := "Income"
		if total < 0 {
			category = "Expense"
		}
		fmt.Printf("  [%s] %s: %.2f\n", tag, category, total)
	}
	// Create a slice of tag-total pairs
	// type tagTotal struct {
	// 	tag   string
	// 	total float64
	// }
	// tagTotals := make([]tagTotal, 0, len(tagSums))
	// for tag, total := range tagSums {
	// 	tagTotals = append(tagTotals, tagTotal{tag, total})
	// }
	//
	// // Sort by total in descending order
	// sort.Slice(tagTotals, func(i, j int) bool {
	// 	return tagTotals[i].total > tagTotals[j].total
	// })
	//
	// // Print sorted results
	// for _, tt := range tagTotals {
	// 	category := "Income"
	// 	if tt.total < 0 {
	// 		category = "Expense"
	// 	}
	// 	fmt.Printf("  [%s] %s: %.2f\n", tt.tag, category, tt.total)
	// }
}

func printHighImpactTags(transactions []Transaction, topN int) {
	type tagStats struct {
		Tag       string
		Total     float64
		Count     int
		AvgPerTxn float64
	}

	tagData := make(map[string]*tagStats)

	for _, txn := range transactions {
		if txn.Amount >= 0 {
			continue // Skip income
		}
		tags := txn.Tags
		if len(tags) == 0 {
			tags = []string{"_untagged_"}
		}
		for _, tag := range tags {
			if stat, exists := tagData[tag]; exists {
				stat.Total += txn.Amount
				stat.Count++
			} else {
				tagData[tag] = &tagStats{
					Tag:   tag,
					Total: txn.Amount,
					Count: 1,
				}
			}
		}
	}

	// Convert map to slice
	var stats []tagStats
	for _, stat := range tagData {
		stat.AvgPerTxn = stat.Total / float64(stat.Count)
		stats = append(stats, *stat)
	}

	// Sort by most negative total (highest impact)
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Total < stats[j].Total
	})

	// Print top N
	if len(stats) > 0 {
		fmt.Println("💸 Top Expense Tags (High Impact):")
		limit := topN
		if len(stats) < topN {
			limit = len(stats)
		}
		for i := 0; i < limit; i++ {
			s := stats[i]
			fmt.Printf("  [%s] Total: %.2f | Count: %d | Avg: %.2f\n", s.Tag, s.Total, s.Count, s.AvgPerTxn)
		}
		fmt.Println()
	}
}

type Projection struct {
	Original  []Transaction
	Projected []Transaction
	AdjustMap map[string]float64
}

func buildProjection(original []Transaction, adjust string) Projection {
	adjustMap := parseAdjustments(adjust)

	var projected []Transaction

	for _, txn := range original {

		adjustedTxn := txn

		// If an inline projected amount is given, use it directly
		if txn.ProjectedAmount != nil {
			// Preserve original sign
			adjustedTxn.Amount = float64(signum(txn.Amount)) * (*txn.ProjectedAmount)
		} else {
			// Apply tag-based adjustment
			for _, tag := range txn.Tags {
				if adj, ok := adjustMap[tag]; ok {
					adjustedTxn.Amount *= (1.0 + adj)
					break
				}
			}
		}

		projected = append(projected, adjustedTxn)

	}

	return Projection{
		Original:  original,
		Projected: projected,
		AdjustMap: adjustMap,
	}
}

func parseAdjustments(s string) map[string]float64 {
	out := map[string]float64{}
	for _, entry := range strings.Split(s, ",") {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		val, err := strconv.ParseFloat(parts[1], 64)
		if err == nil {
			out[strings.TrimSpace(parts[0])] = val
		}
	}
	return out
}

func parseRemovals(s string) map[string]bool {
	out := map[string]bool{}
	for _, tag := range strings.Split(s, ",") {
		tag = strings.TrimSpace(strings.ToLower(tag))
		if tag != "" {
			out[tag] = true
		}
	}
	return out
}

func hasAnyTag(txn Transaction, tagSet map[string]bool) bool {
	for _, tag := range txn.Tags {
		if tagSet[strings.ToLower(tag)] {
			return true
		}
	}
	return false
}

func printSideBySide(p Projection) {
	fmt.Println("📊 Side-by-Side Summary (Original → Projected)")

	origIncome, origExpense := totalAmounts(p.Original)
	projIncome, projExpense := totalAmounts(p.Projected)

	fmt.Printf("\n  Income:    %8.2f  →  %8.2f\n", origIncome, projIncome)
	fmt.Printf("  Expenses:  %8.2f  →  %8.2f\n", -origExpense, -projExpense)
	fmt.Printf("  Net:       %8.2f  →  %8.2f\n\n", origIncome+origExpense, projIncome+projExpense)

	fmt.Println("🔍 Tag Changes:")
	origByTag := tagTotals(p.Original)
	projByTag := tagTotals(p.Projected)

	tagSet := map[string]bool{}
	for tag := range origByTag {
		tagSet[tag] = true
	}
	for tag := range projByTag {
		tagSet[tag] = true
	}

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	for _, tag := range tags {
		o, ok1 := origByTag[tag]
		p, ok2 := projByTag[tag]
		if !ok1 {
			fmt.Printf("  [%s] added:    %.2f\n", tag, p)
		} else if !ok2 {
			fmt.Printf("  [%s] removed:  %.2f\n", tag, o)
		} else if o != p {
			fmt.Printf("  [%s] changed:  %.2f → %.2f\n", tag, o, p)
		}
	}

	fmt.Println()
}

func totalAmounts(transactions []Transaction) (income, expenses float64) {
	for _, t := range transactions {
		if t.Amount >= 0 {
			income += t.Amount
		} else {
			expenses += t.Amount
		}
	}
	return
}

func tagTotals(transactions []Transaction) map[string]float64 {
	out := map[string]float64{}
	for _, txn := range transactions {
		if len(txn.Tags) == 0 {
			out["_untagged_"] += txn.Amount
		} else {
			for _, tag := range txn.Tags {
				out[tag] += txn.Amount
			}
		}
	}
	return out
}

func exportProjectionMarkdown(p Projection, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	w := func(format string, args ...interface{}) {
		fmt.Fprintf(f, format, args...)
	}

	w("# 📊 Cash Flow Projection\n\n")
	w("## Summary\n\n")
	origIncome, origExpense := totalAmounts(p.Original)
	projIncome, projExpense := totalAmounts(p.Projected)

	w("| Metric   | Original | Projected |\n")
	w("|----------|----------|-----------|\n")
	w("| Income   | %.2f     | %.2f      |\n", origIncome, projIncome)
	w("| Expenses | %.2f     | %.2f      |\n", -origExpense, -projExpense)
	w("| Net      | %.2f     | %.2f      |\n\n", origIncome+origExpense, projIncome+projExpense)

	w("## Tag Differences\n\n")
	w("| Tag     | Original | Projected |\n")
	w("|---------|----------|-----------|\n")

	origByTag := tagTotals(p.Original)
	projByTag := tagTotals(p.Projected)

	tagSet := map[string]bool{}
	for tag := range origByTag {
		tagSet[tag] = true
	}
	for tag := range projByTag {
		tagSet[tag] = true
	}

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	for _, tag := range tags {
		o, ok1 := origByTag[tag]
		p, ok2 := projByTag[tag]
		if !ok1 {
			w("| %s | – | %.2f |\n", tag, p)
		} else if !ok2 {
			w("| %s | %.2f | – |\n", tag, o)
		} else if o != p {
			w("| %s | %.2f | %.2f |\n", tag, o, p)
		}
	}

	// Compute high impact tags (expenses only)
	type tagStats struct {
		Tag       string
		Total     float64
		Count     int
		AvgPerTxn float64
	}

	tagData := make(map[string]*tagStats)

	for _, txn := range p.Original {
		if txn.Amount >= 0 {
			continue // Skip income
		}
		tags := txn.Tags
		if len(tags) == 0 {
			tags = []string{"_untagged_"}
		}
		for _, tag := range tags {
			if stat, exists := tagData[tag]; exists {
				stat.Total += txn.Amount
				stat.Count++
			} else {
				tagData[tag] = &tagStats{
					Tag:   tag,
					Total: txn.Amount,
					Count: 1,
				}
			}
		}
	}

	var stats []tagStats
	for _, stat := range tagData {
		stat.AvgPerTxn = stat.Total / float64(stat.Count)
		stats = append(stats, *stat)
	}

	// Sort by most negative total
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Total < stats[j].Total
	})

	w("\n## 💸 Top Expense Tags (High Impact)\n\n")
	w("| Tag | Total | Count | Avg per Transaction |\n")
	w("|-----|--------|--------|---------------------|\n")

	topN := 10
	if len(stats) < topN {
		topN = len(stats)
	}
	for i := 0; i < topN; i++ {
		s := stats[i]
		w("| %s | %.2f | %d | %.2f |\n", s.Tag, s.Total, s.Count, s.AvgPerTxn)
	}

	// w("\n## Transactions by Date\n\n")
	//
	// // Group transactions by date
	// byDate := map[string][]struct {
	// 	Original  Transaction
	// 	Projected Transaction
	// }{}
	//
	// for i := range p.Original {
	// 	dateStr := p.Original[i].Date.Format("2006-01-02")
	// 	byDate[dateStr] = append(byDate[dateStr], struct {
	// 		Original  Transaction
	// 		Projected Transaction
	// 	}{
	// 		Original:  p.Original[i],
	// 		Projected: p.Projected[i],
	// 	})
	// }
	//
	// // Sorted date keys
	// var dates []string
	// for d := range byDate {
	// 	dates = append(dates, d)
	// }
	// sort.Strings(dates)
	//
	// for _, date := range dates {
	// 	w("### %s\n\n", date)
	// 	w("| Description | Original | Projected | Tags |\n")
	// 	w("|-------------|----------|-----------|------|\n")
	//
	// 	for _, pair := range byDate[date] {
	// 		o := pair.Original
	// 		pj := pair.Projected
	//
	// 		// Use projected amount if it differs
	// 		tags := strings.Join(o.Tags, ", ")
	// 		w("| %s | %.2f | %.2f | %s |\n",
	// 			o.Description,
	// 			abs(o.Amount),
	// 			abs(pj.Amount),
	// 			tags,
	// 		)
	// 	}
	// 	w("\n")
	// }

	return nil
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func signum(v float64) int {
	if v < 0 {
		return -1
	}
	return 1
}
