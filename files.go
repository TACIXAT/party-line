package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mitchellh/go-homedir"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

/*
	on start / refresh / monitor?
	walk directory
	load own packs

	advertise own packs

	other packs -> signatures in some recent time frame

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

var sharedDir string

var fullPacks map[string][]*Pack

func init() {
	home, err := homedir.Dir()
	if err != nil {
		log.Fatal("could not get home dir")
	}
	sharedDir = filepath.Join(home, "party-line")
	os.MkdirAll(sharedDir, 0700)
	fullPacks = make(map[string][]*Pack)
}

func sha256File(targetFile *os.File) (string, error) {
	_, err := targetFile.Seek(0, 0)
	if err != nil {
		setStatus("error could not seek to start of file")
		log.Println(err)
		return "", err
	}

	h := sha256.New()
	if _, err := io.Copy(h, targetFile); err != nil {
		setStatus("error could not read file for hash")
		log.Println(err)
		return "", err
	}

	sha256 := fmt.Sprintf("%x", h.Sum(nil))
	return sha256, nil
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

func unpackFile(targetFile *os.File) (*PackFile, error) {
	_, err := targetFile.Seek(0, 0)
	if err != nil {
		setStatus("error seek to start of file for unpack")
		log.Println(err)
		return nil, err
	}

	contents, err := ioutil.ReadAll(targetFile)
	if err != nil {
		setStatus("error could not read file for unpack")
		log.Println(err)
		return nil, err
	}

	packFile := new(PackFile)
	err = json.Unmarshal(contents, packFile)
	if err != nil {
		setStatus("error could not unmarshal json for unpack")
		log.Println(err)
		return nil, err
	}

	return packFile, nil
}

func calculateChain(targetFile *os.File, size int64) (string, error) {
	if size < 0 {
		setStatus("error file size less than 0 (c'est une pipe?)")
		return "", errors.New("file size less than 0")
	}

	var BUFFER_SIZE int64 = 10240

	var prev *Block
	prev = nil

	blocks := make(map[string]*Block)
	lastBlockSize := size % BUFFER_SIZE
	index := size / BUFFER_SIZE

	_, err := targetFile.Seek(-lastBlockSize, 2)
	if err != nil {
		log.Println(err)
		setStatus("error seek failed in file")
		return "", errors.New("seek failed for file")
	}

	// read backward
	for index > -1 {
		buffer := make([]byte, BUFFER_SIZE) // 10 KiB
		bytesRead, err := targetFile.Read(buffer)
		if err != nil && err != io.EOF {
			log.Println(err)
			setStatus("error failed read")
			return "", errors.New("failed read of file")
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
		log.Println(err)
		setStatus("error could not seek to beginning of file")
		return "", errors.New("could not seek to beginning of file")
	}

	// test forward
	currBlockHash := sha256Block(prev)
	for index := 0; true; index++ {
		buffer := make([]byte, BUFFER_SIZE) // 10 KiB
		bytesRead, err := targetFile.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Println(err)
				setStatus("error could not read file (verify)")
				return "", errors.New("could not read file (verify)")
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
			log.Println("Bad hash at " + strconv.FormatInt(int64(index), 10))
			setStatus("error verify failed")
			return "", errors.New("verify failed")
		}

		currBlockHash = curr.NextBlockHash
	}

	return sha256Block(prev), nil
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

func buildPack(partyId string, path string, targetFile *os.File) {
	partyDir := filepath.Join(sharedDir, partyId)

	pack := new(Pack)

	packFile, err := unpackFile(targetFile)
	if err != nil {
		log.Println(err)
		setStatus("error unpacking pack file")
		return
	}

	if len(packFile.Files) == 0 {
		setStatus("error no files in pack")
		return
	}

	pack.Name = packFile.Name

	dirPath := filepath.Dir(path)
	for _, shortFilePath := range packFile.Files {
		sharedFilePath := filepath.Join(dirPath, shortFilePath)
		sharedFilePathAbs, err := filepath.Abs(sharedFilePath)
		if err != nil {
			log.Println(err)
			setStatus("error could not get absolute path for file")
			return
		}

		if !strings.HasPrefix(sharedFilePathAbs, partyDir) {
			setStatus("error pack file outside of channel dir")
			return
		}

		relativePath := sharedFilePathAbs[len(partyDir):]
		relativePath = strings.TrimLeft(relativePath, "/")

		sharedFile, err := os.Open(sharedFilePathAbs)
		if err != nil {
			setStatus("error opening file in pack")
			log.Println(err)
			return
		}

		fileInfo, err := sharedFile.Stat()
		if err != nil {
			setStatus("error getting file info for file in pack")
			log.Println(err)
			return
		}

		fileHash, err := sha256File(sharedFile)
		if err != nil {
			log.Println(err)
			setStatus("error hashing shared file")
			return
		}

		sharedFileSize := fileInfo.Size()
		firstBlockHash, err := calculateChain(sharedFile, sharedFileSize)
		if err != nil {
			log.Println(err)
			setStatus(err.Error())
			return
		}

		packFileInfo := new(PackFileInfo)
		packFileInfo.Name = relativePath
		packFileInfo.Path = sharedFilePathAbs
		packFileInfo.Hash = fileHash
		packFileInfo.FirstBlockHash = firstBlockHash
		packFileInfo.Size = sharedFileSize
		packFileInfo.Coverage = fullCoverage(packFileInfo.Size)

		pack.Files = append(pack.Files, packFileInfo)
	}

	fullPacks[partyId] = append(fullPacks[partyId], pack)
}

func walker(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	relPath := path[len(sharedDir):]
	relPath = strings.TrimLeft(relPath, "/")
	toks := strings.Split(relPath, "/")
	if len(toks) < 1 {
		setStatus("error bad path when walking")
		return nil
	}

	partyId := toks[0]
	chatStatus(partyId)

	if !info.IsDir() {
		targetFile, err := os.Open(path)
		if err != nil {
			setStatus("could not open files")
			return nil
		}

		defer targetFile.Close()

		if !strings.HasSuffix(path, ".pack") {
			return nil
		}

		buildPack(partyId, path, targetFile)
	}
	return nil
}

func runOccasionally() {
	for partyId, _ := range parties {
		targetDir := filepath.Join(sharedDir, partyId)
		chatStatus(targetDir)
		_, err := os.Stat(targetDir)
		if err != nil {
			continue
		}

		err = filepath.Walk(targetDir, walker)

	}
}
