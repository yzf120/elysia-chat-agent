package test

import "fmt"

func knapsack(weights []int, values []int, capacity int) int {
	n := len(weights)
	if n == 0 {
		return 0
	}

	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, capacity+1)
	}

	for i := 1; i <= n; i++ {
		for w := 0; w <= capacity; w++ {
			weight := weights[i-1]
			value := values[i-1]

			if weight > w {
				dp[i][w] = dp[i-1][w]
			} else {
				dp[i][w] = max(dp[i-1][w], value+dp[i-1][w-weight])
			}
		}
	}

	return dp[n][capacity]
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	weights := []int{0, 2, 3}
	values := []int{10, 4, 5}
	capacity := 0

	result := knapsack(weights, values, capacity)
	fmt.Printf("weights = %v\n", weights)
	fmt.Printf("values  = %v\n", values)
	fmt.Printf("capacity = %d\n", capacity)
	fmt.Printf("max value = %d\n", result)
}
