// 哑 HTTP git 裸仓构建器：把导出目录树打包成可被 `git clone` 的文件集，
// 由服务端直接对外提供（/marketplace.git），无需任何 git 托管。
// 仅用标准库实现 git 对象模型（blob/tree/commit + 松散对象 + info/refs）。
package marketplace

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

// BuildGitMarketplace 返回 裸仓相对路径 → 文件内容 的全集。
// parent 非空时形成真实提交链，远端 pull 才能识别增量（否则全新根提交会被
// 判为"已是最新"）；进程首次构建无 parent。
func BuildGitMarketplace(tree map[string][]byte, parent *[20]byte, executable ...map[string]bool) (map[string][]byte, error) {
	repo := map[string][]byte{}
	store := func(objectType string, content []byte) [20]byte {
		full := append([]byte(fmt.Sprintf("%s %d\x00", objectType, len(content))), content...)
		sum := sha1.Sum(full)
		var compressed bytes.Buffer
		writer := zlib.NewWriter(&compressed)
		_, _ = writer.Write(full)
		_ = writer.Close()
		repo[fmt.Sprintf("objects/%s/%s", hex.EncodeToString(sum[:1]), hex.EncodeToString(sum[1:]))] = compressed.Bytes()
		return sum
	}
	root := &gitTreeNode{files: map[string]gitBlob{}, dirs: map[string]*gitTreeNode{}}
	for path, content := range tree {
		parts := strings.Split(strings.Trim(path, "/"), "/")
		node := root
		for _, segment := range parts[:len(parts)-1] {
			child, ok := node.dirs[segment]
			if !ok {
				child = &gitTreeNode{files: map[string]gitBlob{}, dirs: map[string]*gitTreeNode{}}
				node.dirs[segment] = child
			}
			node = child
		}
		node.files[parts[len(parts)-1]] = gitBlob{content: content, executable: len(executable) > 0 && executable[0][path]}
	}
	treeSHA := writeGitTree(root, store)
	timestamp := time.Now().UTC().Unix()
	identity := fmt.Sprintf("Loadout Marketplace <marketplace@loadout.dev> %d +0000", timestamp)
	commit := fmt.Sprintf("tree %s\n", hex.EncodeToString(treeSHA[:]))
	if parent != nil {
		commit += fmt.Sprintf("parent %s\n", hex.EncodeToString(parent[:]))
	}
	commit += fmt.Sprintf("author %s\ncommitter %s\n\nLoadout marketplace snapshot\n", identity, identity)
	commitSHA := store("commit", []byte(commit))
	head := hex.EncodeToString(commitSHA[:])
	repo["HEAD"] = []byte("ref: refs/heads/main\n")
	repo["refs/heads/main"] = []byte(head + "\n")
	// 哑协议：不带 service 参数的 /info/refs 逐行列出引用。
	repo["info/refs"] = []byte(head + "\trefs/heads/main\n")
	repo["objects/info/packs"] = []byte("")
	repo["config"] = []byte("[core]\n\trepositoryformatversion = 0\n\tfilemode = true\n\tbare = true\n")
	repo["description"] = []byte("Loadout marketplace (served live)\n")
	return repo, nil
}

type gitBlob struct {
	executable bool
	content    []byte
	sum        [20]byte
	stored     bool
}

type gitTreeNode struct {
	files map[string]gitBlob
	dirs  map[string]*gitTreeNode
}

// writeGitTree 递归序列化 git tree 对象；排序遵循 git 规则（目录名按追加 '/' 比较）。
func writeGitTree(node *gitTreeNode, store func(string, []byte) [20]byte) [20]byte {
	names := make([]string, 0, len(node.files)+len(node.dirs))
	for name := range node.files {
		names = append(names, name)
	}
	for name := range node.dirs {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		a, b := names[i], names[j]
		if _, dir := node.dirs[a]; dir {
			a += "/"
		}
		if _, dir := node.dirs[b]; dir {
			b += "/"
		}
		return a < b
	})
	var serialized bytes.Buffer
	for _, name := range names {
		if blob, ok := node.files[name]; ok {
			if !blob.stored {
				blob.sum = store("blob", blob.content)
				blob.stored = true
				node.files[name] = blob
			}
			mode := "100644"
			if blob.executable {
				mode = "100755"
			}
			serialized.WriteString(mode + " " + name + "\x00")
			serialized.Write(blob.sum[:])
			continue
		}
		childSHA := writeGitTree(node.dirs[name], store)
		serialized.WriteString("40000 " + name + "\x00")
		serialized.Write(childSHA[:])
	}
	return store("tree", serialized.Bytes())
}
