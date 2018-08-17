package main

import (
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"time"
)

type Block struct {
	Next int
	Skip int
}

type Chain struct {
	Coverage []int
}

const BUFFER_SIZE = 10240

const (
	B int = 1 << (10 * iota)
	KB
	MB
	GB
	TB
)

func left(i int) int {
	return 2*i + 1
}

func right(i int) int {
	return 2*i + 2
}

func parent(i int) int {
	return (i - 1) / 2
}

func getIndices(length int) {
	for i := 0; i < length; i++ {
		skip_left := 2*i + 1
		skip_right := 2*i + 2
		if i == (length - 1) {
			fmt.Println(i, "_", "_")
		} else if skip_left < length && skip_right < length {
			fmt.Println(i, i+1, skip_left, skip_right)
		} else if skip_left < length && skip_right >= length {
			fmt.Println(i, i+1, skip_left, "_")
		} else {
			fmt.Println(i, i+1, "_", "_")
		}
	}
}

func fullCoverage(size int) *big.Int {
	coverage := new(big.Int)

	for i := 0; i*BUFFER_SIZE < int(size); i++ {
		coverage.SetBit(coverage, i, 1)
	}

	return coverage
}

func hasBlock(coverage *big.Int, idx int) bool {
	return coverage.Bit(idx) == 1
}

func randomCoverage(size int, covered int) *big.Int {
	coverage := new(big.Int)
	coverage.SetBit(coverage, 0, 1)
	for i := 0; i < covered; i++ {
		idx := rand.Intn(size / BUFFER_SIZE)
		for coverage.Bit(idx) == 1 {
			idx = rand.Intn(size / BUFFER_SIZE)
		}

		coverage.SetBit(coverage, idx, 1)
	}

	return coverage
}

func maxSteps(size int) int {
	return int(math.Log(float64(size)))
}

func findPath(coverage *big.Int, idx int) []int {
	result := make([]int, 0)
	currIdx := idx
	for currIdx != 0 {
		result = append(result, currIdx)
		currIdx = parent(currIdx)
	}

	result = append(result, currIdx)
	return result
}

func pathTo(coverage *big.Int, idx int) []int {
	return nil
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	blocks := 100
	size := blocks * BUFFER_SIZE
	getIndices(blocks)
	fmt.Printf("%b\n", fullCoverage(size))

	fmt.Println("50KB", maxSteps(50*KB))
	fmt.Println("700MB", maxSteps(700*MB))
	fmt.Println("10GB", maxSteps(10*GB))
	randCov := randomCoverage(size, 10)
	fmt.Printf("%0100b\n", randCov)
	fmt.Println(findPath(randCov, 99))
	fmt.Println("setting")
	randCovTB := randomCoverage(TB, 1000)
	fmt.Println("searching")
	fmt.Println(findPath(randCovTB, TB/BUFFER_SIZE))
}

/*
MB = (1 << (10 * 2))
GB = (1 << (10 * 3))
TB = (1 << (10 * 4))
BLOCK = 10240
(MB / BLOCK) / 3 **
*/
