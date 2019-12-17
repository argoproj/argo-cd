import {describeNode, ResourceTreeNode} from "./application-resource-tree";

test("describeNode.NoImages", () => {
    expect(describeNode({
        kind: "my-kind",
        name: "my-name",
        namespace: "my-ns",
    } as ResourceTreeNode)).toBe(`Kind: my-kind
Namespace: my-ns
Name: my-name`)
});

test("describeNode.Images", () => {
    expect(describeNode({
        kind: "my-kind",
        name: "my-name",
        namespace: "my-ns",
        images: ['my-image:v1'],
    } as ResourceTreeNode)).toBe(`Kind: my-kind
Namespace: my-ns
Name: my-name
Images:
- my-image:v1`)
});