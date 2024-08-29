# Commit Server

The Argo CD Commit Server provides push access to git repositories for hydrated manifests.

The server exposes a gRPC service which accepts requests to push hydrated manifests to a git repository. This is the interface:

```protobuf
// CommitManifests represents the caller's request for some Kubernetes manifests to be pushed to a git repository.
message CommitManifests {
  // repoURL is the URL of the repo we're pushing to. HTTPS or SSH URLs are acceptable.
  required string repoURL = 1;
  // targetBranch is the name of the branch we're pushing to.
  required string targetBranch = 2;
  // drySHA is the full SHA256 hash of the "dry commit" from which the manifests were hydrated.
  required string drySHA = 3;
  // commitAuthor is the name of the author of the dry commit.
  required string commitAuthor = 4;
  // commitMessage is the short commit message from the dry commit.
  required string commitMessage = 5;
  // commitTime is the dry commit timestamp.
  required string commitTime = 6;
  // details holds the information about the actual hydrated manifests.
  repeated CommitPathDetails details = 7;
}

// CommitManifestDetails represents the details about a 
message CommitPathDetails {
  // path is the path to the directory to which these manifests should be written.
  required string path = 1;
  // manifests is a list of JSON documents representing the Kubernetes manifests.
  repeated string manifests = 2;
  // readme is a string which will be written to a README.md alongside the manifest.yaml. 
  required string readme = 3;
}

message CommitManifestsResponse {
}
```
