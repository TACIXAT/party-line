package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

/*
	peers send signed packs
	client show packs with sig counts
	user selects a pack
	user requests pack
	downloads/party-line/channel is created if not present
	*https://github.com/mitchellh/go-homedir os.Join(homedir.Dir(), "party-line", channel)
	pieces come in
	client constructs chains
		map[hash of block] block
		as blocks are verified they are written to disk
*/

type PackFile struct {
	Name  string
	Files []string
}

type PackFileInfo struct {
	Name           string
	Path           string
	Hash           string
	FirstBlockHash string
	Size           int64
	Coverage       []uint64
}

type Pack struct {
	Name  string
	Files []*PackFileInfo
}

type Block struct {
	Index         uint64
	NextBlockHash string
	Data          []byte
	DataHash      string
}

var sharedDirAbs string

func sha256File(targetFile *os.File) string {
	_, err := targetFile.Seek(0, 0)
	if err != nil {
		log.Fatal(err)
	}

	h := sha256.New()
	if _, err := io.Copy(h, targetFile); err != nil {
		log.Fatal(err)
	}

	sha256 := fmt.Sprintf("%x", h.Sum(nil))
	return sha256
}

func sha256Bytes(buffer []byte) string {
	sum := sha256.Sum256(buffer)
	return fmt.Sprintf("%x", sum)
}

func sha256Block(block *Block) string {
	if block == nil {
		return ""
	}

	hash := sha256.New()
	hash.Write([]byte(strconv.FormatUint(block.Index, 10)))
	hash.Write([]byte(block.NextBlockHash))
	hash.Write([]byte(block.DataHash))
	hash.Write(block.Data)
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func unpackFile(targetFile *os.File) *PackFile {
	_, err := targetFile.Seek(0, 0)
	if err != nil {
		log.Fatal(err)
	}

	contents, err := ioutil.ReadAll(targetFile)
	if err != nil {
		log.Fatal(err)
	}

	packFile := new(PackFile)
	err = json.Unmarshal(contents, packFile)
	if err != nil {
		log.Fatal(err)
	}

	return packFile
}

func calculateChain(targetFile *os.File, size int64) string {
	if size < 0 {
		log.Fatal("size lt 0")
	}

	var BUFFER_SIZE int64 = 10240

	var prev *Block
	prev = nil

	blocks := make(map[string]*Block)
	lastBlockSize := size % BUFFER_SIZE
	index := size / BUFFER_SIZE

	_, err := targetFile.Seek(-lastBlockSize, 2)
	if err != nil {
		log.Fatal(err)
	}

	// read backward
	for index > -1 {
		buffer := make([]byte, BUFFER_SIZE) // 10 KiB
		bytesRead, err := targetFile.Read(buffer)
		if err != nil && err != io.EOF {
			log.Fatal(err)
		}

		sha256Buffer := sha256Bytes(buffer[:bytesRead])

		curr := new(Block)
		curr.Index = uint64(index)
		curr.Data = buffer[:bytesRead]
		curr.DataHash = sha256Buffer
		curr.NextBlockHash = sha256Block(prev)

		blockHash := sha256Block(curr)
		blocks[blockHash] = curr

		prev = curr
		index--
		_, err = targetFile.Seek(-(int64(bytesRead) + BUFFER_SIZE), 1)
	}

	_, err = targetFile.Seek(0, 0)
	if err != nil {
		log.Fatal(err)
	}

	// test forward
	currBlockHash := sha256Block(prev)
	for index := 0; true; index++ {
		buffer := make([]byte, BUFFER_SIZE) // 10 KiB
		bytesRead, err := targetFile.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Fatal(err)
			}
			break
		}

		sha256Buffer := sha256Bytes(buffer[:bytesRead])

		curr := new(Block)
		curr.Index = uint64(index)
		curr.Data = buffer[:bytesRead]
		curr.DataHash = sha256Buffer
		curr.NextBlockHash = blocks[currBlockHash].NextBlockHash

		verifyBlockHash := sha256Block(curr)
		if verifyBlockHash != currBlockHash {
			log.Fatal("Bad hash at " + strconv.FormatInt(int64(index), 10))
		}

		currBlockHash = curr.NextBlockHash
	}

	return sha256Block(prev)
}

func fullCoverage(size int64) []uint64 {
	coverage := make([]uint64, 0)

	var BUFFER_SIZE uint64 = 10240
	var curr uint64 = 0
	var i uint64 = 0
	for i = 0; i*BUFFER_SIZE < uint64(size); i++ {
		curr |= 1 << (i % 64)
		if (i+1)%64 == 0 {
			fmt.Println("adding", i)
			coverage = append(coverage, curr)
			curr = 0
		}
	}

	if curr != 0 {
		coverage = append(coverage, curr)
	}

	return coverage
}

func buildPack(path string, targetFile *os.File) {
	pack := new(Pack)

	packFile := unpackFile(targetFile)

	if len(packFile.Files) == 0 {
		log.Fatal("no files in pack")
	}

	pack.Name = packFile.Name

	dirPath := filepath.Dir(path)
	for _, shortFilePath := range packFile.Files {
		sharedFilePath := filepath.Join(dirPath, shortFilePath)
		sharedFilePathAbs, err := filepath.Abs(sharedFilePath)
		if err != nil {
			log.Fatal(err)
		}

		if !strings.HasPrefix(sharedFilePathAbs, sharedDirAbs) {
			log.Fatal(
				"bad pack " + packFile.Name +
					" file outside of file dir " + sharedFilePathAbs)
		}

		relativePath := sharedFilePathAbs[len(sharedDirAbs):]
		relativePath = strings.TrimLeft(relativePath, "/")

		sharedFile, err := os.Open(sharedFilePathAbs)
		if err != nil {
			log.Fatal(err)
		}

		fileInfo, err := sharedFile.Stat()
		if err != nil {
			log.Fatal(err)
		}

		fileHash := sha256File(sharedFile)
		sharedFileSize := fileInfo.Size()
		firstBlockHash := calculateChain(sharedFile, sharedFileSize)

		packFileInfo := new(PackFileInfo)
		packFileInfo.Name = relativePath
		packFileInfo.Path = sharedFilePathAbs
		packFileInfo.Hash = fileHash
		packFileInfo.FirstBlockHash = firstBlockHash
		packFileInfo.Size = sharedFileSize
		packFileInfo.Coverage = fullCoverage(packFileInfo.Size)

		pack.Files = append(pack.Files, packFileInfo)
	}

	fmt.Println("=== PACK ===\n")
	fmt.Println("Name:", pack.Name, "\n")
	for i := 0; i < len(pack.Files); i++ {
		fmt.Println("Name:", pack.Files[i].Name)
		fmt.Println("Hash:", pack.Files[i].Hash)
		fmt.Println("First:", pack.Files[i].FirstBlockHash)
		fmt.Println("Size:", pack.Files[i].Size)
		fmt.Println("Path:", pack.Files[i].Path)
		fmt.Println("Coverage:", pack.Files[i].Coverage)
		fmt.Println()
	}
}

func walker(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	if !info.IsDir() {
		targetFile, err := os.Open(path)
		if err != nil {
			log.Fatal(err)
		}
		defer targetFile.Close()

		if !strings.HasSuffix(path, ".pack") {
			return nil
		}

		buildPack(path, targetFile)
	}
	return nil
}

func main() {
	fileDir := "shared"
	var err error
	sharedDirAbs, err = filepath.Abs(fileDir)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("target:", sharedDirAbs)
	err = filepath.Walk(sharedDirAbs, walker)

	if err != nil {
		log.Println(err)
	}
}
