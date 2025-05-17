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
    Date        time.Time
    Type        string
    Amount      float64
    Description string
    Tags        []string
}

// CLI flags
var (
    filterTag  string
    filterType string
    fromDate   string
    toDate     string
)

func init() {
    flag.StringVar(&filterTag, "tag", "", "Filter transactions by tag")
    flag.StringVar(&filterType, "type", "", "Filter by type: income or expense")
    flag.StringVar(&fromDate, "from", "", "Start date YYYY-MM-DD")
    flag.StringVar(&toDate, "to", "", "End date YYYY-MM-DD")
}

func main() {
    flag.Parse()

    transactions, err := parseSimpleMarkdown("sample-cashflow.md")
    if err != nil {
        fmt.Println("Error:", err)
        return
    }

    transactions = applyFilters(transactions)
    printSummary(transactions)
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
    txnRegex := regexp.MustCompile(`^([+-])\s*([\d.]+)\s+(.+?)(?:\s+\[(.+)\])?$`)

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

        if matches := txnRegex.FindStringSubmatch(line); len(matches) >= 4 {
            sign := matches[1]
            amount, err := strconv.ParseFloat(matches[2], 64)
            if err != nil {
                continue
            }
            if sign == "-" {
                amount = -amount
            }

            description := matches[3]
            tags := []string{}
            if len(matches) == 5 && matches[4] != "" {
                tags = strings.Split(matches[4], ",")
                for i := range tags {
                    tags[i] = strings.TrimSpace(tags[i])
                }
            }

            transactions = append(transactions, Transaction{
                Date:        currentDate,
                Type:        map[bool]string{true: "income", false: "expense"}[amount >= 0],
                Amount:      amount,
                Description: description,
                Tags:        tags,
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

    for _, txn := range transactions {
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
}

