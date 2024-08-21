package dir

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0glabs/0g-storage-client/core"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
)

// FileType represents the file type in the FsNode structure.
type FileType string

const (
	File      FileType = "file"
	Directory FileType = "directory"
	Symbolic  FileType = "symbolic"
)

// FsNode represents a node in the filesystem hierarchy.
type FsNode struct {
	Name    string      `json:"name"`              // File or directory name
	Type    FileType    `json:"type"`              // File type of the node
	Hash    common.Hash `json:"hash,omitempty"`    // Merkle hash
	Size    int64       `json:"size,omitempty"`    // File size in bytes (only for files)
	Link    string      `json:"link,omitempty"`    // Symbolic link target
	Entries []*FsNode   `json:"entries,omitempty"` // Directory entries (only for directories)
}

// NewDirFsNode creates a new FsNode representing a directory.
func NewDirFsNode(name string, entryNodes []*FsNode) *FsNode {
	sort.Slice(entryNodes, func(i, j int) bool {
		return entryNodes[i].Name < entryNodes[j].Name
	})

	rootHash := calculateRootHash(entryNodes)

	return &FsNode{
		Name:    name,
		Type:    Directory,
		Hash:    rootHash,
		Entries: entryNodes,
	}
}

// calculateRootHash computes the Merkle root hash for a list of FsNode entries.
func calculateRootHash(entries []*FsNode) common.Hash {
	var root common.Hash

	if len(entries) == 0 {
		return root
	}

	root = crypto.Keccak256Hash(entries[len(entries)-1].Hash[:])
	for i := len(entries) - 2; i >= 0; i-- {
		root = crypto.Keccak256Hash(entries[i].Hash[:], root[:])
	}

	return root
}

// NewFileFsNode creates a new FsNode representing a file.
func NewFileFsNode(name string, hash common.Hash, size int64) *FsNode {
	return &FsNode{
		Name: name,
		Type: File,
		Hash: hash,
		Size: size,
	}
}

// NewSymbolicFsNode creates a new FsNode representing a symbolic link.
func NewSymbolicFsNode(name, link string) *FsNode {
	return &FsNode{
		Name: name,
		Type: Symbolic,
		Hash: crypto.Keccak256Hash([]byte(link)),
		Link: link,
	}
}

// Search looks for a file by name in the current directory node's entries.
func (node *FsNode) Search(fileName string) (*FsNode, bool) {
	i, found := sort.Find(len(node.Entries), func(i int) int {
		return strings.Compare(fileName, node.Entries[i].Name)
	})

	if found {
		return node.Entries[i], true
	}
	return nil, false
}

// BuildFileTree recursively builds a file tree for the specified directory.
func BuildFileTree(path string) (*FsNode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to stat file %s", path)
	}

	if !info.IsDir() {
		return nil, errors.New("file tree building is only supported for directory")
	}

	root, err := build(path)
	if err != nil {
		return nil, err
	}

	// Root directory represented as "."
	root.Name = "."
	return root, nil
}

// build is a helper function that recursively builds a file tree starting from the specified path.
func build(path string) (*FsNode, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to stat file %s", path)
	}

	switch {
	case info.IsDir():
		return buildDirectoryNode(path, info)
	case info.Mode()&os.ModeSymlink != 0:
		return buildSymbolicNode(path, info)
	case info.Mode().IsRegular():
		return buildFileNode(path, info)
	default:
		return nil, errors.New("unsupported file type")
	}
}

// buildDirectoryNode creates an FsNode for a directory, including its contents.
func buildDirectoryNode(path string, info os.FileInfo) (*FsNode, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to read directory %s", path)
	}

	var entryNodes []*FsNode
	for _, entry := range entries {
		entryPath := filepath.Join(path, entry.Name())
		entryNode, err := build(entryPath)
		if err != nil {
			return nil, err
		}
		entryNodes = append(entryNodes, entryNode)
	}
	return NewDirFsNode(info.Name(), entryNodes), nil
}

// buildSymbolicNode creates an FsNode for a symbolic link.
func buildSymbolicNode(path string, info os.FileInfo) (*FsNode, error) {
	link, err := os.Readlink(path)
	if err != nil {
		return nil, errors.WithMessagef(err, "invalid symbolic link %s", path)
	}

	return NewSymbolicFsNode(info.Name(), link), nil
}

// buildFileNode creates an FsNode for a regular file, including its Merkle root hash.
func buildFileNode(path string, info os.FileInfo) (*FsNode, error) {
	hash, err := core.MerkleRoot(path)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to calculate merkle root for %s", path)
	}
	return NewFileFsNode(info.Name(), hash, info.Size()), nil
}
