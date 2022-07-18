- 从cmd/main.go开始
  - cmd/argocd是argocd命令行的代码

## 打包
- 需要Go 1.18 
### 版本信息
- Makefile
  ```text
  override LDFLAGS += \
    -X ${PACKAGE}.version=${VERSION} \
    -X ${PACKAGE}.buildDate=${BUILD_DATE} \
    -X ${PACKAGE}.gitCommit=${GIT_COMMIT} \
    -X ${PACKAGE}.gitTreeState=${GIT_TREE_STATE}\
    -X ${PACKAGE}.kubectlVersion=${KUBECTL_VERSION}
  ```
- 代码: common/version.go

## cmd/argocd-server
> 作用就是: 如何与外部交互?
### grpc如何实现的 
### 如何打包这个组件
- make mod-vendor-local
- make server
- 打包的输出文件: dist/argocd-server
- 版本信息: common/version.go


