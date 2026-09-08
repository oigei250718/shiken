BINARY  := shiken
IMAGE   := shiken
TAG     := latest
PORT    := 8080
# 部署时注入容器的 MySQL 连接串，可用 make deploy DSN=... 覆盖
DSN     ?= root:123456@tcp(100.108.142.7:3306)/shiken?charset=utf8mb4&multiStatements=true

.PHONY: build clean docker deploy undeploy

# 编译本地可执行文件（macOS）
build:
	CGO_ENABLED=0 go build -o $(BINARY) .

# 清理构建产物（不触碰数据库与备份文件）
clean:
	rm -f $(BINARY)

# 构建 Docker 镜像
docker:
	docker build -t $(IMAGE):$(TAG) .
	@echo "镜像构建完成: $(IMAGE):$(TAG)"
	@echo "运行示例: docker run --rm -p $(PORT):$(PORT) -e SHIKEN_MYSQL_DSN='$(DSN)' $(IMAGE):$(TAG)"

# 一键部署：构建带时间戳的新镜像 -> 停掉并移除旧容器 -> 删除旧镜像 -> 启动新容器
deploy:
	@set -e; \
	tag=$$(date +%Y%m%d%H%M%S); \
	echo "==> [1/4] 构建镜像 $(IMAGE):$$tag"; \
	docker build -t $(IMAGE):$$tag .; \
	echo "==> [2/4] 清理旧容器"; \
	if docker ps -a --format '{{.Names}}' | grep -qx shiken; then \
		docker stop shiken; \
		docker rm shiken; \
	else \
		echo "无运行中的 shiken 容器，跳过"; \
	fi; \
	echo "==> [3/4] 清理旧镜像"; \
	old=$$(docker images $(IMAGE) --format '{{.Repository}}:{{.Tag}}' | grep -v ":$$tag" || true); \
	if [ -n "$$old" ]; then \
		echo "移除: $$old"; \
		docker rmi $$old || true; \
	else \
		echo "无旧镜像，跳过"; \
	fi; \
	echo "==> [4/4] 启动容器"; \
	docker run -d --name shiken -p $(PORT):$(PORT) -e SHIKEN_MYSQL_DSN='$(DSN)' $(IMAGE):$$tag; \
	echo "部署完成: $(IMAGE):$$tag -> http://localhost:$(PORT)"

# 停止并移除 shiken 容器（保留镜像和数据）
undeploy:
	@if docker ps -a --format '{{.Names}}' | grep -qx shiken; then \
		docker stop shiken; \
		docker rm shiken; \
		echo "shiken 容器已停止并移除"; \
	else \
		echo "未发现 shiken 容器，无需操作"; \
	fi
