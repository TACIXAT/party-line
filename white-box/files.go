package whitebox

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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
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
	pieces come in
	client constructs chains
		map[hash of block] block
		as blocks are verified they are written to disk
*/

type DotPack struct {
	Name  string
	Files []string
}

type PackFileInfo struct {
	Name           string
	Hash           string
	FirstBlockHash string
	Size           int64
	BlockMap       map[string]BlockInfo `json:"-"`
	BlockLookup    map[uint64]string    `json:"-"`
	Coverage       []uint64             `json:"-"`
	Path           string               `json:"-"`
}

const (
	AVAILABLE = iota
	ACTIVE
	COMPLETE
)

type Pack struct {
	Name     string
	Files    []*PackFileInfo
	State    int                  `json:"-"`
	Peers    map[string]time.Time `json:"-"`
	FileLock *sync.Mutex          `json:"-"`
}

type PendingPack struct {
	Name  string
	Hash  string
	Files []PendingFile
}

// mirror of PackFileInfo so we can serialize to disko
type PendingFile struct {
	Name           string
	Hash           string
	Size           int64
	FirstBlockHash string
	Coverage       []uint64
	BlockMap       map[string]BlockInfo
	BlockLookup    map[uint64]string
	Path           string
}

// this struct / function is a little silly
// it only exists because we don't serialize BlockMap, Coverage, and Path
// over the network, so now when we want to serialize to disk we need to
// copy to another struct, if anyone has better ideas open an issue
func (pack *Pack) ToPendingPack() *PendingPack {
	pendingPack := new(PendingPack)
	pendingPack.Name = pack.Name
	pendingPack.Hash = sha256Pack(pack)

	pack.FileLock.Lock()
	for _, file := range pack.Files {
		pendingFile := new(PendingFile)
		pendingFile.Name = file.Name
		pendingFile.Hash = file.Hash
		pendingFile.Size = file.Size
		pendingFile.FirstBlockHash = file.FirstBlockHash
		pendingFile.BlockMap = file.BlockMap
		pendingFile.BlockLookup = file.BlockLookup
		pendingFile.Coverage = file.Coverage
		pendingFile.Path = file.Path
		pendingPack.Files = append(pendingPack.Files, *pendingFile)
	}
	pack.FileLock.Unlock()

	return pendingPack
}

func (pack *Pack) GetFileInfo(fileHash string) *PackFileInfo {
	for _, packFileInfo := range pack.Files {
		if packFileInfo.Hash == fileHash {
			return packFileInfo
		}
	}

	return nil
}

func (pack *Pack) SetPaths(baseDir string) {
	for _, file := range pack.Files {
		path := filepath.Join(baseDir, file.Name)
		if !strings.HasPrefix(path, baseDir) {
			errMsg := "error skipping file " + pack.Name + "for dir traversal"
			log.Println(errMsg)
			continue
		}
		file.Path = path
	}
}

type BlockInfo struct {
	Index          uint64
	NextBlockHash  string
	LeftBlockHash  string
	RightBlockHash string
	DataHash       string
}

type Block struct {
	Index          uint64
	NextBlockHash  string
	LeftBlockHash  string
	RightBlockHash string
	Data           []byte
	DataHash       string
}

func (block *Block) ToBlockInfo() *BlockInfo {
	blockInfo := new(BlockInfo)
	blockInfo.Index = block.Index
	blockInfo.NextBlockHash = block.NextBlockHash
	blockInfo.LeftBlockHash = block.LeftBlockHash
	blockInfo.RightBlockHash = block.RightBlockHash
	blockInfo.DataHash = block.DataHash
	return blockInfo
}

func (blockInfo *BlockInfo) ToBlock(data []byte) *Block {
	block := new(Block)
	block.Index = blockInfo.Index
	block.NextBlockHash = blockInfo.NextBlockHash
	block.LeftBlockHash = blockInfo.LeftBlockHash
	block.RightBlockHash = blockInfo.RightBlockHash
	block.DataHash = blockInfo.DataHash
	block.Data = data
	return block
}

func buildBlockLookup(
	blockMap map[string]BlockInfo, firstBlockHash string) map[uint64]string {
	currBlockHash := firstBlockHash
	var idx uint64 = 0
	blockLookup := make(map[uint64]string)
	for currBlockHash != "" {
		blockLookup[idx] = currBlockHash
		currBlockHash = blockMap[currBlockHash].NextBlockHash
		idx++
	}
	return blockLookup
}

const BUFFER_SIZE = 10240

func (wb *WhiteBox) InitFiles(dir string) {
	if dir == "" {
		home, err := homedir.Dir()
		if err != nil {
			log.Fatal("could not get home dir")
		}
		wb.SharedDir = filepath.Join(home, "party-line")
	} else {
		wb.SharedDir = dir
	}

	err := os.MkdirAll(wb.SharedDir, 0700)
	if err != nil {
		log.Fatal("could not create shared dir")
	}
}

type ByFileName []*PackFileInfo

func (packFileInfo ByFileName) Len() int {
	return len(packFileInfo)
}

func (packFileInfo ByFileName) Swap(i, j int) {
	packFileInfo[i], packFileInfo[j] = packFileInfo[j], packFileInfo[i]
}

func (packFileInfo ByFileName) Less(i, j int) bool {
	return packFileInfo[i].Name < packFileInfo[j].Name
}

func sha256Pack(pack *Pack) string {
	if pack == nil {
		return ""
	}

	sort.Sort(ByFileName(pack.Files))

	hash := sha256.New()
	hash.Write([]byte(pack.Name))
	for _, packFileInfo := range pack.Files {
		hash.Write([]byte(packFileInfo.Name))
		hash.Write([]byte(packFileInfo.Hash))
		hash.Write([]byte(packFileInfo.FirstBlockHash))
		hash.Write([]byte(strconv.FormatInt(packFileInfo.Size, 10)))
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func (wb *WhiteBox) sha256File(targetFile *os.File) (string, error) {
	_, err := targetFile.Seek(0, 0)
	if err != nil {
		wb.setStatus("error could not seek to start of file")
		log.Println(err)
		return "", err
	}

	h := sha256.New()
	if _, err := io.Copy(h, targetFile); err != nil {
		wb.setStatus("error could not read file for hash")
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
	hash.Write([]byte(block.LeftBlockHash))
	hash.Write([]byte(block.RightBlockHash))
	hash.Write([]byte(block.DataHash))
	hash.Write(block.Data)
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func (wb *WhiteBox) unpackFile(targetFile *os.File) (*DotPack, error) {
	_, err := targetFile.Seek(0, 0)
	if err != nil {
		wb.setStatus("error seek to start of file for unpack")
		log.Println(err)
		return nil, err
	}

	contents, err := ioutil.ReadAll(targetFile)
	if err != nil {
		wb.setStatus("error could not read file for unpack")
		log.Println(err)
		return nil, err
	}

	dotPack := new(DotPack)
	err = json.Unmarshal(contents, dotPack)
	if err != nil {
		wb.setStatus("error could not unmarshal json for unpack")
		log.Println(err)
		return nil, err
	}

	return dotPack, nil
}

func leftParent(i uint64) uint64 {
	return (i - 1) / 2
}

func rightParent(i uint64) uint64 {
	return (i - 2) / 2
}

func treeParent(i uint64) uint64 {
	// I think left parent works for both cases cause math
	if i%2 == 1 {
		return leftParent(i)
	}

	return rightParent(i)
}

func leftChild(i uint64) uint64 {
	return 2*i + 1
}

func rightChild(i uint64) uint64 {
	return 2*i + 2
}

func (wb *WhiteBox) calculateChain(
	targetFile *os.File, size int64) (string, map[string]BlockInfo, error) {
	if size < 0 {
		wb.setStatus("error file size less than 0 (c'est une pipe?)")
		return "", nil, errors.New("file size less than 0")
	}

	var prev *Block
	prev = nil

	blocks := make(map[string]*Block)
	blockMap := make(map[string]BlockInfo)
	lastBlockSize := size % BUFFER_SIZE
	index := size / BUFFER_SIZE

	_, err := targetFile.Seek(-lastBlockSize, 2)
	if err != nil {
		log.Println(err)
		wb.setStatus("error seek failed in file")
		return "", nil, errors.New("seek failed for file")
	}

	skips := make(map[uint64]string)

	// read backward
	for index > -1 {
		buffer := make([]byte, BUFFER_SIZE) // 10 KiB
		bytesRead, err := targetFile.Read(buffer)
		if err != nil && err != io.EOF {
			log.Println(err)
			wb.setStatus("error failed read")
			return "", nil, errors.New("failed read of file")
		}

		sha256Buffer := sha256Bytes(buffer[:bytesRead])

		curr := new(Block)
		curr.Index = uint64(index)
		curr.Data = buffer[:bytesRead]
		curr.DataHash = sha256Buffer
		curr.NextBlockHash = sha256Block(prev)
		curr.LeftBlockHash = skips[leftChild(curr.Index)]
		curr.RightBlockHash = skips[rightChild(curr.Index)]

		blockHash := sha256Block(curr)
		blocks[blockHash] = curr
		blockMap[blockHash] = *curr.ToBlockInfo()

		prev = curr
		skips[curr.Index] = blockHash

		index--
		_, err = targetFile.Seek(-(int64(bytesRead) + BUFFER_SIZE), 1)
	}

	_, err = targetFile.Seek(0, 0)
	if err != nil {
		log.Println(err)
		wb.setStatus("error could not seek to beginning of file")
		return "", nil, errors.New("could not seek to beginning of file")
	}

	// test forward
	currBlockHash := sha256Block(prev)
	for index := 0; true; index++ {
		buffer := make([]byte, BUFFER_SIZE) // 10 KiB
		bytesRead, err := targetFile.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Println(err)
				wb.setStatus("error could not read file (verify)")
				return "", nil, errors.New("could not read file (verify)")
			}
			break
		}

		sha256Buffer := sha256Bytes(buffer[:bytesRead])

		curr := new(Block)
		curr.Index = uint64(index)
		curr.Data = buffer[:bytesRead]
		curr.DataHash = sha256Buffer
		curr.NextBlockHash = blocks[currBlockHash].NextBlockHash
		curr.LeftBlockHash = blocks[currBlockHash].LeftBlockHash
		curr.RightBlockHash = blocks[currBlockHash].RightBlockHash

		verifyBlockHash := sha256Block(curr)
		if verifyBlockHash != currBlockHash {
			log.Println("Bad hash at " + strconv.FormatInt(int64(index), 10))
			wb.setStatus("error verify failed")
			return "", nil, errors.New("verify failed")
		}

		currBlockHash = curr.NextBlockHash
	}

	return sha256Block(prev), blockMap, nil
}

func isEmptyCoverage(coverage []uint64) bool {
	for i := range coverage {
		if coverage[i] != 0 {
			return false
		}
	}

	return true
}

func emptyCoverage(size int64) []uint64 {
	count := uint64(size) / (BUFFER_SIZE * 64)
	if uint64(size)%(BUFFER_SIZE*64) != 0 {
		count++
	}
	coverage := make([]uint64, count)
	return coverage
}

func isFullCoverage(size int64, coverage []uint64) bool {
	compare := fullCoverage(size)

	if coverage == nil || compare == nil {
		panic("coverage for full coverage nil")
	}

	if len(coverage) != len(compare) {
		panic("coverage lne != full coverage len")
	}

	for i := range coverage {
		if coverage[i] != compare[i] {
			return false
		}
	}

	return true
}

func fullCoverage(size int64) []uint64 {
	coverage := make([]uint64, 0)

	var curr uint64 = 0
	var i uint64 = 0
	for i = 0; i*BUFFER_SIZE < uint64(size); i++ {
		curr |= 1 << (i % 64)
		if (i+1)%64 == 0 {
			coverage = append(coverage, curr)
			curr = 0
		}
	}

	if curr != 0 {
		coverage = append(coverage, curr)
	}

	return coverage
}

func (wb *WhiteBox) buildPack(partyId string, path string, targetFile *os.File) {
	partyDir := filepath.Join(wb.SharedDir, partyId)
	partyDirAbs, err := filepath.Abs(partyDir)
	if err != nil {
		log.Println(err)
		wb.setStatus("error could not get absolute path for party dir")
		return
	}

	if !strings.HasPrefix(partyDirAbs, wb.SharedDir) {
		wb.setStatus("error party dir traverses directories")
		return
	}

	pack := new(Pack)
	pack.Peers = make(map[string]time.Time)
	pack.Peers[wb.PeerSelf.Id()] = time.Now().UTC()
	pack.State = COMPLETE
	pack.Files = make([]*PackFileInfo, 0)
	pack.FileLock = new(sync.Mutex)

	dotPack, err := wb.unpackFile(targetFile)
	if err != nil {
		log.Println(err)
		wb.setStatus("error unpacking pack file")
		return
	}

	if len(dotPack.Files) == 0 {
		wb.setStatus("error no files in pack")
		return
	}

	pack.Name = dotPack.Name

	dirPath := filepath.Dir(path)
	for _, shortFilePath := range dotPack.Files {
		sharedFilePath := filepath.Join(dirPath, shortFilePath)
		sharedFilePathAbs, err := filepath.Abs(sharedFilePath)
		if err != nil {
			log.Println(err)
			wb.setStatus("error could not get absolute path for file")
			return
		}

		if !strings.HasPrefix(sharedFilePathAbs, partyDir) {
			wb.setStatus("error pack file outside of channel dir")
			return
		}

		relativePath := sharedFilePathAbs[len(partyDir):]
		relativePath = strings.TrimLeft(relativePath, "/")

		sharedFile, err := os.Open(sharedFilePathAbs)
		if err != nil {
			wb.setStatus("error opening file in pack")
			log.Println(err)
			return
		}

		fileInfo, err := sharedFile.Stat()
		if err != nil {
			wb.setStatus("error getting file info for file in pack")
			log.Println(err)
			return
		}

		fileHash, err := wb.sha256File(sharedFile)
		if err != nil {
			log.Println(err)
			wb.setStatus("error hashing shared file")
			return
		}

		sharedFileSize := fileInfo.Size()
		firstBlockHash, blockMap, err :=
			wb.calculateChain(sharedFile, sharedFileSize)
		if err != nil {
			log.Println(err)
			wb.setStatus(err.Error())
			return
		}

		packFileInfo := new(PackFileInfo)
		packFileInfo.Name = relativePath
		packFileInfo.Path = sharedFilePathAbs
		packFileInfo.Hash = fileHash
		packFileInfo.FirstBlockHash = firstBlockHash
		packFileInfo.Size = sharedFileSize
		packFileInfo.Coverage = fullCoverage(packFileInfo.Size)
		packFileInfo.BlockMap = blockMap
		packFileInfo.BlockLookup = buildBlockLookup(blockMap, firstBlockHash)

		pack.Files = append(pack.Files, packFileInfo)
	}

	sort.Sort(ByFileName(pack.Files))
	packHash := sha256Pack(pack)

	wb.Parties[partyId].Packs[packHash] = pack
}

func (wb *WhiteBox) walker(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	relPath := path[len(wb.SharedDir):]
	relPath = strings.TrimLeft(relPath, "/")
	toks := strings.Split(relPath, "/")
	if len(toks) < 1 {
		wb.setStatus("error bad path when walking")
		return nil
	}

	partyId := toks[0]
	if !info.IsDir() {
		targetFile, err := os.Open(path)
		if err != nil {
			wb.setStatus("could not open files")
			return nil
		}

		defer targetFile.Close()

		if !strings.HasSuffix(path, ".pack") {
			return nil
		}

		wb.buildPack(partyId, path, targetFile)
	}
	return nil
}

func (wb *WhiteBox) RescanPacks() {
	// check for new and changed packs
	for partyId, party := range wb.Parties {
		party.ClearPacks()
		targetDir := filepath.Join(wb.SharedDir, partyId)

		_, err := os.Stat(targetDir)
		if err != nil {
			continue
		}

		err = filepath.Walk(targetDir, wb.walker)
	}

	wb.advertiseAll()
}
