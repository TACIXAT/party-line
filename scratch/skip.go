package main 

import (
	"math/big"
	"math/rand"
	"math"
	"fmt"
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
     B  int = 1 << (10 * iota)
     KB 
     MB
     GB
     TB
 )

func getIndices(length int) {
	for i := 0; i < length; i++ {
		skip := i+2*i
		if skip < length && skip > i {
			fmt.Println(i, i+1, skip)
		} else {
			fmt.Println(i, i+1, "_")
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
	if size < BUFFER_SIZE {
		return 0
	}

	steps := math.Log(float64(size / BUFFER_SIZE)) / math.Log(float64(3)) 
	return int(steps)
}

func findNearest(coverage *big.Int, idx int, depth int) []int {
	if depth > 20 {
		return nil
	}

	if coverage.Bit(idx) == 1 {
		result := make([]int, 0)
		return append(result, idx)
	}

	path1 := findNearest(coverage, idx / 3, depth+1)
	path2 := findNearest(coverage, idx - 1, depth+1)

	if len(path1) < len(path2) {
		return append(path1, idx)
	}

	return append(path2, idx)
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

	fmt.Println("50KB", maxSteps(50 * KB))
	fmt.Println("700MB", maxSteps(700 * MB))
	fmt.Println("10GB", maxSteps(10 * GB))
	randCov := randomCoverage(size, 10)
	fmt.Printf("%0100b\n", randCov)
	fmt.Println(findNearest(randCov, 99, 0))
	fmt.Println("setting")
	randCovTB := randomCoverage(TB, 1000)
	fmt.Println("searching")
	fmt.Println(findNearest(randCovTB, TB / BUFFER_SIZE, 0))
}

/*
MB = (1 << (10 * 2)) 
GB = (1 << (10 * 3)) 
TB = (1 << (10 * 4)) 
BLOCK = 10240
(MB / BLOCK) / 3 **  
*/
