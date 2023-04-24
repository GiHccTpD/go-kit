# Go Kit

## version

### 简介
一个支持通过支持 `--version` 的命令来支持输出当前当前构建的git版本等信息

### 使用

1. 将以下 `shell` 脚本放到 `scrpit` 文件夹下
```shell
#!/usr/bin/env bash

OUTPUT_DIR="_output"

{
  if [ ! -n "$1" ] ;then
      echo "需要指定输出的名称!"
      exit 1
  else
      echo "指定的名称为: $1"
  fi

  # 目前仅支持此名称与cmd已存在的服务入口文件夹一致
  server_name=$1

  # 定义版本相关变量

  ## 指定应用使用的 version 包，会通过 `-ldflags -X` 向该包中指定的变量注入值
  VERSION_PACKAGE=github.com/GiHccTpD/go-kit/version

  ## 定义 VERSION 语义化版本号
  if [ -z "$VERSION" ]; then
    VERSION="$(git describe --tags --always --match='v*')"
  fi

  ## 检查代码仓库是否是 dirty（默认dirty）
  GIT_TREE_STATE="dirty"
  if [ -z "$(git status --porcelain 2>/dev/null)" ]; then
    GIT_TREE_STATE="clean"
  fi
  GIT_COMMIT="$(git rev-parse HEAD)"

  GO_LDFLAGS+=" -X ${VERSION_PACKAGE}.GitVersion=${VERSION}"
  GO_LDFLAGS+=" -X ${VERSION_PACKAGE}.GitCommit=${GIT_COMMIT}"
  GO_LDFLAGS+=" -X ${VERSION_PACKAGE}.GitTreeState=${GIT_TREE_STATE}"
  GO_LDFLAGS+=" -X ${VERSION_PACKAGE}.BuildDate=$(date -u +'%Y-%m-%dT%H:%M:%S')"

  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -ldflags "$GO_LDFLAGS" -o ../$OUTPUT_DIR/$server_name ../cmd/$server_name/main.go
}
```
2. 执行改脚本
3. 执行build产物
```bash
./your_server_name --version
  gitVersion: v0.1.0-1-gc1f3278                       
   gitCommit: c1f3278f9848ca31df70b7ad6828f1fe379a5688
gitTreeState: dirty                                   
   buildDate: 2023-04-24T02:52:28                     
   goVersion: go1.20.2                                
    compiler: gc                                      
    platform: darwin/arm64  
```
