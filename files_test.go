package main

import "testing"

func TestLeftChild(t *testing.T) {
	tables := []struct {
		in  uint64
		out uint64
	}{
		{0, 1},
		{1, 3},
		{100, 201},
		{4294967295, 8589934591},
	}

	for _, table := range tables {
		left := leftChild(table.in)
		if left != table.out {
			t.Errorf("Incorrect child for index %d: %d", table.in, table.out)
		}
	}
}

func TestRightChild(t *testing.T) {
	tables := []struct {
		in  uint64
		out uint64
	}{
		{0, 2},
		{1, 4},
		{100, 202},
		{4294967295, 8589934592},
	}

	for _, table := range tables {
		right := rightChild(table.in)
		if right != table.out {
			t.Errorf("Incorrect child for index %d: %d", table.in, table.out)
		}
	}
}