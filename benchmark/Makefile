.PHONY: all build-mac build-linux clean test run

# 默认目标
all: build-mac

# 构建Mac可执行文件
build-mac:
	@echo "构建Mac可执行文件..."
	GOOS=darwin GOARCH=amd64 go build -o bin/benchmark-mac cmd/benchmark.go
	@echo "✓ 构建完成: bin/benchmark-mac"

# 构建Linux可执行文件
build-linux:
	@echo "构建Linux可执行文件..."
	GOOS=linux GOARCH=amd64 go build -o bin/benchmark-linux cmd/benchmark.go
	@echo "✓ 构建完成: bin/benchmark-linux"

# 构建所有平台
build-all: build-mac build-linux

# 清理构建文件
clean:
	@echo "清理构建文件..."
	rm -rf bin/
	rm -rf reports/*.xlsx
	@echo "✓ 清理完成"

# 运行测试（需要先构建）
run: build-mac
	@echo "运行基准测试..."
	./bin/benchmark-mac

# 运行Go测试
test:
	@echo "运行单元测试..."
	go test ./... -v

# 安装依赖
deps:
	@echo "安装依赖..."
	go mod download
	go mod tidy
	@echo "✓ 依赖安装完成"

# 运行示例测试（小规模）
demo: build-mac
	@echo "运行演示测试（小规模）..."
	./bin/benchmark-mac --count 10 --mode single

# 运行完整测试
full: build-mac
	@echo "运行完整基准测试..."
	./bin/benchmark-mac --mode all
