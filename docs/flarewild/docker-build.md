# Docker 构建步骤

## 前置条件

- Docker 已安装（建议 20.10+）
- 当前目录结构：

```
/home/mark_/workspace/
├── open-im-server/          # 本项目（已修改）
└── openimsdk-tools/         # 本地 clone 的 tools 依赖（已修改）
```

- `go.mod` 中已有 replace 指令：

```
replace github.com/openimsdk/tools => ../openimsdk-tools
```

## 构建方式：go mod vendor

由于 `go.mod` 中使用了 `replace` 指向本地路径 `../openimsdk-tools`，Docker build context 无法直接访问项目外的目录。因此需要先通过 `go mod vendor` 将所有依赖（含修改后的 tools）复制到项目内的 `vendor/` 目录，再构建镜像。

### Step 1: 生成 vendor 目录

```bash
cd /home/mark_/workspace/open-im-server
go mod vendor
```

执行后会在项目根目录生成 `vendor/` 目录，其中包含修改后的 `openimsdk/tools` 代码。

可通过以下命令验证 CloudFront 改动已包含在 vendor 中：

```bash
grep -n "cloudFrontURL\|CloudFrontURL" vendor/github.com/openimsdk/tools/s3/aws/aws.go
```

应能看到 `Config` 和 `Aws` 结构体中的 CloudFrontURL 字段。

### Step 2: 构建并推送 Docker 镜像（ARM64）

目标部署机器为 ARM 架构，使用 `docker buildx` 进行跨平台构建并直接推送到 ECR：

<!-- ```bash
docker buildx build --platform linux/arm64 \
  -t 608291746647.dkr.ecr.us-east-1.amazonaws.com/vt/openim-server:latest \
  --push .
``` -->

如需指定版本标签：

```bash
docker buildx build --platform linux/arm64 \
  -t 608291746647.dkr.ecr.us-east-1.amazonaws.com/vt/openim-server:v3.8.3-cloudfront \
  --push .
```

> 说明：
> - 使用 `--platform linux/arm64` 交叉编译为 ARM64 架构
> - `--push` 构建完成后直接推送到 ECR，不保留本地镜像
> - 推送前需先登录 ECR：`aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin 608291746647.dkr.ecr.us-east-1.amazonaws.com`
> - 现有 Dockerfile 使用 `COPY . .` 将整个项目复制到构建容器中。当 `vendor/` 目录存在时，`go mod download` 不会再从网络下载已 vendor 的依赖，构建过程会自动使用 vendor 中修改后的 tools 代码

### Step 3: 验证镜像

在 ARM64 部署机器上拉取镜像后验证：

```bash
docker run --rm 608291746647.dkr.ecr.us-east-1.amazonaws.com/vt/openim-server:latest \
  ls /openim-server/_output/bin/platforms/linux/arm64/
```

应能看到包括 `openim-rpc-third` 在内的所有二进制文件。

## 部署配置

构建好的镜像中，配置文件位于 `/openim-server/config/openim-rpc-third.yml`。

部署时需要通过挂载配置文件或环境变量方式设置 AWS + CloudFront 参数：

```yaml
object:
  enable: aws
  aws:
    region: ap-southeast-2
    bucket: <your-bucket>
    accessKeyID: <your-key>
    secretAccessKey: <your-secret>
    sessionToken:
    publicRead: false
    cloudFrontURL: https://media.flarewild.com
```

示例挂载方式：

```bash
docker run -d \
  -v /path/to/your/config:/openim-server/config \
  --name openim-server \
  608291746647.dkr.ecr.us-east-1.amazonaws.com/vt/openim-server:latest
```

## 清理（可选）

构建完成后如不再需要本地 vendor 目录，可清理：

```bash
rm -rf vendor/
```

## 注意事项

- 每次修改 `openimsdk-tools` 中的代码后，需要重新执行 `go mod vendor` 再构建镜像
- `vendor/` 目录较大，建议在 `.gitignore` 中忽略（通常已包含）
- Dockerfile 已修改为：有 `vendor/` 目录时跳过 `go mod download`，并通过 `GOFLAGS=-mod=vendor` 强制使用 vendor 模式编译

