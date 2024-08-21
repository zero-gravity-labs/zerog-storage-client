package dir_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0glabs/0g-storage-client/core"
	"github.com/0glabs/0g-storage-client/transfer/dir"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

func TestNewDirFsNode(t *testing.T) {
	child1 := &dir.FsNode{Name: "child1", Hash: common.HexToHash("0x01")}
	child2 := &dir.FsNode{Name: "child2", Hash: common.HexToHash("0x02")}
	children := []*dir.FsNode{child1, child2}

	node := dir.NewDirFsNode("root", children)

	assert.Equal(t, "root", node.Name)
	assert.Equal(t, dir.Directory, node.Type)
	assert.NotEqual(t, common.Hash{}, node.Hash)
	assert.Len(t, node.Entries, 2)
	assert.Equal(t, "child1", node.Entries[0].Name)
	assert.Equal(t, "child2", node.Entries[1].Name)
}

func TestNewFileFsNode(t *testing.T) {
	hash := common.HexToHash("0x12345")
	node := dir.NewFileFsNode("file.txt", hash, 1024)

	assert.Equal(t, "file.txt", node.Name)
	assert.Equal(t, dir.File, node.Type)
	assert.Equal(t, hash, node.Hash)
	assert.Equal(t, int64(1024), node.Size)
}

func TestNewSymbolicFsNode(t *testing.T) {
	link := "/some/path"
	node := dir.NewSymbolicFsNode("symlink", link)

	assert.Equal(t, "symlink", node.Name)
	assert.Equal(t, dir.Symbolic, node.Type)
	assert.Equal(t, crypto.Keccak256Hash([]byte(link)), node.Hash)
	assert.Equal(t, link, node.Link)
}

func TestSearch(t *testing.T) {
	child1 := &dir.FsNode{Name: "child1"}
	child2 := &dir.FsNode{Name: "child2"}
	children := []*dir.FsNode{child1, child2}

	node := dir.NewDirFsNode("root", children)

	result, found := node.Search("child1")
	assert.True(t, found)
	assert.Equal(t, child1, result)

	result, found = node.Search("nonexistent")
	assert.False(t, found)
	assert.Nil(t, result)
}

func TestBuildFileTree(t *testing.T) {
	tempDir := t.TempDir()

	// create test file
	filePath := filepath.Join(tempDir, "testfile.txt")
	err := os.WriteFile(filePath, []byte("content"), 0644)
	assert.NoError(t, err)

	// create symbolic link
	linkPath := filepath.Join(tempDir, "symlink")
	err = os.Symlink(filePath, linkPath)
	assert.NoError(t, err)

	// Create a subdirectory
	subDirPath := filepath.Join(tempDir, "subdir")
	err = os.Mkdir(subDirPath, 0755)
	assert.NoError(t, err)

	// Create a test file inside the subdirectory
	subDirFilePath := filepath.Join(subDirPath, "subdirfile.txt")
	err = os.WriteFile(subDirFilePath, []byte("subdir content"), 0644)
	assert.NoError(t, err)

	// Build the file tree
	var root *dir.FsNode
	t.Run("test building file tree", func(t *testing.T) {
		root, err = dir.BuildFileTree(tempDir)
		assert.NoError(t, err)
		assert.Equal(t, dir.Directory, root.Type)
		assert.Equal(t, root.Name, ".")
		assert.Len(t, root.Entries, 3) // "testfile.txt", "symlink", "subdir"
	})

	t.Run("test subdir file node", func(t *testing.T) {
		subDirNode, found := root.Search("subdir")
		assert.True(t, found)
		assert.Equal(t, dir.Directory, subDirNode.Type)
		assert.Len(t, subDirNode.Entries, 1) // "subdirfile.txt"

		subDirFileNode, found := subDirNode.Search("subdirfile.txt")
		assert.True(t, found)
		assert.Equal(t, dir.File, subDirFileNode.Type)
	})

	t.Run("test file node", func(t *testing.T) {
		node, found := root.Search("testfile.txt")
		assert.True(t, found)
		// Calculate expected hash using core.MerkleRoot
		expectedHash, err := core.MerkleRoot(filePath)
		assert.NoError(t, err)
		assert.Equal(t, expectedHash, node.Hash)
	})

	t.Run("test symbolic link node", func(t *testing.T) {
		node, found := root.Search("symlink")
		assert.True(t, found)
		assert.Equal(t, dir.Symbolic, node.Type)
		assert.Equal(t, filePath, node.Link)
	})
}
