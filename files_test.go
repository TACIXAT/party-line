package main

import "testing"

func TestSha256Bytes(t *testing.T) {
	data := []byte("party line!")
	expected := "c76a324153f7154b8bbaee8e60c623cc4ec950126f8465a8dc32f901d10e6e01"
	hash := sha256Bytes(data)
	if hash != expected {
		t.Errorf("Hash does not match for sha256Bytes(\"party line!\"):")
		t.Errorf("Got: %s", hash)
		t.Errorf("Expecting: %s", expected)
	}
}

func TestSha256Pack(t *testing.T) {
	pack := new(Pack)
	pack.Files = make([]*PackFileInfo, 0)

	pack.Name = "Test Pack"

	packFileInfo := new(PackFileInfo)
	packFileInfo.Name = "Test File"
	packFileInfo.Hash =
		"b36189cbfe6157aa35416783786b8fefb5eb5c9994f44b9267a519d813a5a15e"
	packFileInfo.FirstBlockHash =
		"1d0fea39ec33ff7543f345be85d1ccd34d6d864297d4151b737802cb294a338c"
	packFileInfo.Size = 0x45
	pack.Files = append(pack.Files, packFileInfo)

	packFileInfo = new(PackFileInfo)
	packFileInfo.Name = "Another Test File"
	packFileInfo.Hash =
		"49ec57aef4a90ed82f4970ce4e6d341efec5098dce1d7678f8c2cffcf72fb250"
	packFileInfo.FirstBlockHash =
		"c5353be4b3bc52507a5a87edcb9d35a3d55bf2da0635fbe440a429f1ceaf7cf8"
	packFileInfo.Size = 0x45
	pack.Files = append(pack.Files, packFileInfo)

	hash := sha256Pack(pack)
	expected := "2ff563fbd95183b83ca5590b4af42a7942112b8dedee108a28796e1b608ecd3d"
	if hash != expected {
		t.Errorf("Hash does not match for sha256Pack:")
		t.Errorf("Got: %s", hash)
		t.Errorf("Expecting: %s", expected)
	}
}

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

func TestEmptyCoverage(t *testing.T) {
	var size int64 = BUFFER_SIZE * 129 // 64 + 64 + 1
	empty := emptyCoverage(size)

	emptyLen := len(empty)
	if emptyLen != 3 {
		t.Errorf("emptyCoverage unexpected length for size %d:", size)
		t.Errorf("Got: %d", emptyLen)
		t.Errorf("Expecting: 3")
	}

	for _, ea := range empty {
		if ea != 0 {
			t.Errorf("emptyCoverage returned non zero!")
		}
	}

	if !isEmptyCoverage(empty) {
		t.Errorf("isEmptyCoverage returned false on generated coverage!")
	}

	empty[1] = 9
	if isEmptyCoverage(empty) {
		t.Errorf("isEmptyCoverage returned true on non-empty coverage!")
	}
}

func TestFullCoverage(t *testing.T) {
	var size int64 = BUFFER_SIZE * 130 // 64 + 64 + 2
	full := fullCoverage(size)

	fullLen := len(full)
	if fullLen != 3 {
		t.Errorf("fullCoverage unexpected length for size %d:", size)
		t.Errorf("Got: %d", fullLen)
		t.Errorf("Expecting: 3")
	}

	expected := make([]uint64, 3)
	expected[0] = 0xFFFFFFFFFFFFFFFF
	expected[1] = 0xFFFFFFFFFFFFFFFF
	expected[2] = 3
	for idx, ea := range full {
		if ea != expected[idx] {
			t.Errorf("fullCoverage has unexpected value at idx: %d", idx)
			t.Errorf("Got: %d", ea)
			t.Errorf("Expecting: %d", expected[idx])
		}
	}

	if !isFullCoverage(size, full) {
		t.Errorf("isFullCoverage returned false on generated coverage!")
	}

	full[1] = 9
	if isFullCoverage(size, full) {
		t.Errorf("isFullCoverage returned true on non-full coverage!")
	}
}
