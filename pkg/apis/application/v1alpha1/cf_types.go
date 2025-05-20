package v1alpha1

func (n *ResourceNode) GetAllChildNodes(tree *ApplicationTree, kind string) []ResourceNode {
	curChildren := []ResourceNode{}

	for _, c := range tree.Nodes {
		if (kind == "" || kind == c.Kind) && c.hasInParents(tree, n) {
			curChildren = append(curChildren, c)
		}
	}

	return curChildren
}

func (n *ResourceNode) hasInParents(tree *ApplicationTree, p *ResourceNode) bool {
	if len(n.ParentRefs) == 0 {
		return false
	}

	for _, curParentRef := range n.ParentRefs {
		if curParentRef.IsEqual(p.ResourceRef) {
			return true
		}

		parentNode := tree.FindNode(curParentRef.Group, curParentRef.Kind, curParentRef.Namespace, curParentRef.Name)
		if parentNode == nil {
			continue
		}

		parentResult := parentNode.hasInParents(tree, p)
		if parentResult {
			return true
		}
	}

	return false
}

func (r ResourceRef) IsEqual(other ResourceRef) bool {
	return (r.Group == other.Group &&
		r.Version == other.Version &&
		r.Kind == other.Kind &&
		r.Namespace == other.Namespace &&
		r.Name == other.Name) ||
		r.UID == other.UID
}
