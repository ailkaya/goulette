package utils

import (
	"testing"
)

func TestHashRing(t *testing.T) {
	ring := NewHashRing(3)

	// 添加节点
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	// 测试获取节点
	node := ring.GetNode("test-topic")
	if node == "" {
		t.Error("应该返回一个节点")
	}

	// 测试获取多个节点
	nodes := ring.GetNodes("test-topic", 2)
	if len(nodes) != 2 {
		t.Errorf("期望2个节点，实际得到%d个", len(nodes))
	}

	// 移除节点
	ring.RemoveNode("node1")

	// 验证节点已被移除
	newNodes := ring.GetNodes("test-topic", 3)
	for _, n := range newNodes {
		if n == "node1" {
			t.Error("node1应该已被移除")
		}
	}
}

func TestNodeIterator(t *testing.T) {
	ring := NewHashRing(3)

	// 添加节点
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	// 测试迭代器
	iter := ring.GetNodeIter("test-topic")

	// 获取第一个节点
	node1, err := iter.Next()
	if err != nil {
		t.Errorf("获取第一个节点时不应该有错误: %v", err)
	}
	if node1 == "" {
		t.Error("迭代器应该返回第一个节点")
	}

	// 获取第二个节点
	node2, err := iter.Next()
	if err != nil {
		t.Errorf("获取第二个节点时不应该有错误: %v", err)
	}
	if node2 == "" {
		t.Error("迭代器应该返回第二个节点")
	}

	// 确保返回的节点不同
	if node1 == node2 {
		t.Error("迭代器应该返回不同的节点")
	}

	// 测试重置
	iter.Reset()
	resetNode1, err := iter.Next()
	if err != nil {
		t.Errorf("重置后获取第一个节点时不应该有错误: %v", err)
	}
	if resetNode1 != node1 {
		t.Error("重置后应该返回相同的第一个节点")
	}

	// 测试获取所有节点
	allNodes := make([]string, 0)
	iter.Reset()
	for {
		node, err := iter.Next()
		if node == "" || err != nil {
			break
		}
		allNodes = append(allNodes, node)
	}

	if len(allNodes) != 3 {
		t.Errorf("期望3个节点，实际得到%d个", len(allNodes))
	}
}

func TestNodeIteratorError(t *testing.T) {
	ring := NewHashRing(3)

	// 添加节点
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	// 测试迭代器错误情况
	iter := ring.GetNodeIter("test-topic")

	// 遍历所有节点
	nodes := make([]string, 0)
	for {
		node, err := iter.Next()
		if err != nil {
			// 应该在有错误时返回最后一个节点
			if node != "" {
				nodes = append(nodes, node)
			}
			break
		}
		if node == "" {
			break
		}
		nodes = append(nodes, node)
	}

	// 应该获取到所有3个节点，并在最后返回错误
	if len(nodes) != 3 {
		t.Errorf("期望3个节点，实际得到%d个", len(nodes))
	}
}

func TestMD5Hash(t *testing.T) {
	data := []byte("test data")
	hash := MD5Hash(data)

	if len(hash) != 16 {
		t.Errorf("MD5哈希长度应该是16字节，实际是%d字节", len(hash))
	}
}
