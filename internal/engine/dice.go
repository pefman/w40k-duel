package engine

import (
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var diceRe = regexp.MustCompile(`(?i)^\s*(\d+)?\s*d\s*(\d+)(\s*([+\-x*])\s*(\d+))?\s*$`)

// rollExpr supports: N, NdM, NdM+K, NdM-K, NdM xK (multiply) / * K
func rollExpr(r *rand.Rand, expr string) int {
    expr = strings.TrimSpace(expr)
    if expr == "" { return 0 }
    // raw int
    if n, err := strconv.Atoi(expr); err == nil {
        return n
    }
    m := diceRe.FindStringSubmatch(expr)
    if m == nil {
        return 0
    }
    count := 1
    if m[1] != "" { count, _ = strconv.Atoi(m[1]) }
    sides, _ := strconv.Atoi(m[2])
    total := 0
    for i := 0; i < count; i++ {
        total += 1 + r.Intn(sides)
    }
    if m[3] != "" {
        op := m[4]
        k, _ := strconv.Atoi(m[5])
        switch op {
        case "+": total += k
        case "-": total -= k
        case "x", "*": total *= k
        }
    }
    if total < 0 { total = 0 }
    return total
}

func newRNG() *rand.Rand { return rand.New(rand.NewSource(time.Now().UnixNano())) }
