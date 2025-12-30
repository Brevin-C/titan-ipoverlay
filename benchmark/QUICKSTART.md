# IP代理性能测试工具 - 快速开始指南

## 立即使用

### 前提条件

1. 确保泰坦代理客户端正在运行（端口1080）
2. 或者修改配置文件指向你的代理服务器

### 快速测试（推荐首次使用）

```bash
cd benchmark

# 10次快速测试（约30秒）
./bin/benchmark-mac --count 10 --mode single --target https://www.google.com

# 查看生成的Excel报告
open reports/benchmark_report.xlsx
```

### 完整测试

```bash
# 使用默认配置运行完整测试（1000次单次 + 多级并发）
./bin/benchmark-mac

# Linux服务器上使用
./bin/benchmark-linux
```

### 自定义测试

```bash
# 测试YouTube（1000次）
./bin/benchmark-mac --target https://www.youtube.com --count 1000

# 只运行并发测试
./bin/benchmark-mac --mode concurrent

# 自定义并发数
./bin/benchmark-mac --mode concurrent --concurrency 50 --count 200
```

## 配置文件

编辑 `configs/bench_config.yaml`：

```yaml
proxies:
  titan:
    socks5: "127.0.0.1:1080"  # 修改为实际代理地址
    name: "泰坦代理"

targets:
  - name: "测试目标"
    url: "https://www.google.com"  # 修改为测试目标
```

## 报告位置

所有测试报告保存在 `reports/` 目录：

- `reports/benchmark_report.xlsx` - Excel格式报告

## 常见问题

**Q: 代理连接失败？**
A: 确认代理服务运行在配置的地址和端口

**Q: DNS解析时间为0？**
A: 使用代理时，DNS由代理服务器完成，客户端无法测量

**Q: 测试太慢？**
A: 使用 `--count 10` 减少请求数，或 `--mode single` 只运行单次测试

## 更多信息

查看完整文档：`README.md`
