NAME := $(shell jq -r .name manifest.json 2>/dev/null || echo my-template)
BINARY := presto-template-$(NAME)

# Go 安全白名单：只允许这些标准库前缀
# 第三方库不受此限制（由 go.mod 管理，审查依赖即可）
GO_STDLIB_DENY := ^net$$|^net/|^os/exec$$|^plugin$$|^debug/|^testing$$

# 当前平台构建
build:
	go build -o $(BINARY) .

# 安装到 Presto 并预览
preview: build
	mkdir -p ~/.presto/templates/$(NAME)
	cp $(BINARY) ~/.presto/templates/$(NAME)/$(BINARY)
	./$(BINARY) --manifest > ~/.presto/templates/$(NAME)/manifest.json
	@echo "✓ 模板已安装到 ~/.presto/templates/$(NAME)/"
	@echo "  请在 Presto 中刷新模板列表查看效果"

# 运行测试
test: build test-security
	@echo "Testing manifest..."
	@./$(BINARY) --manifest | python3 -m json.tool > /dev/null
	@echo "Testing example round-trip..."
	@./$(BINARY) --example | ./$(BINARY) > /dev/null
	@echo "Testing version..."
	@./$(BINARY) --version > /dev/null
	@# 校验 category 字段
	@./$(BINARY) --manifest | python3 -c "\
import json, sys, re; \
m = json.load(sys.stdin); \
cat = m.get('category', ''); \
(not cat) and (print('ERROR: category is empty', file=sys.stderr) or sys.exit(1)); \
(len(cat) > 20) and (print(f'ERROR: category too long ({len(cat)} > 20)', file=sys.stderr) or sys.exit(1)); \
(not re.match(r'^[\u4e00-\u9fff\w\s-]+$$', cat)) and (print(f'ERROR: category contains invalid characters: {cat}', file=sys.stderr) or sys.exit(1)); \
print(f'  category: {cat} ✓')"
	@echo "All tests passed."

# 安全测试
test-security: build
	@echo "==> Security: static import analysis..."
	@FORBIDDEN=$$(go list -f '{{join .Imports "\n"}}' ./... 2>/dev/null | grep -E '$(GO_STDLIB_DENY)'); \
	if [ -n "$$FORBIDDEN" ]; then \
		echo "SECURITY FAIL: forbidden stdlib imports found:"; echo "$$FORBIDDEN"; exit 1; \
	fi
	@echo "  static analysis ✓"
	@echo "==> Security: network isolation test..."
	@if command -v sandbox-exec >/dev/null 2>&1; then \
		echo "# Test" | sandbox-exec -p '(version 1)(allow default)(deny network*)' ./$(BINARY) > /dev/null && \
		echo "  sandbox-exec (macOS) ✓"; \
	elif unshare --net true 2>/dev/null; then \
		echo "# Test" | unshare --net ./$(BINARY) > /dev/null && \
		echo "  unshare --net (Linux) ✓"; \
	else \
		echo "  SKIP: no sandbox tool available (install sandbox-exec or unshare)"; \
	fi
	@echo "==> Security: output format validation..."
	@OUTPUT=$$(./$(BINARY) --example | ./$(BINARY)); \
	if echo "$$OUTPUT" | grep -qiE '<html|<script|<iframe|<img|<link|<!DOCTYPE|<div|<span'; then \
		echo "SECURITY FAIL: stdout contains HTML"; exit 1; \
	fi; \
	if ! echo "$$OUTPUT" | head -1 | grep -q '^#'; then \
		echo "SECURITY FAIL: stdout first line does not start with Typst directive"; exit 1; \
	fi
	@echo "  output validation ✓"

# 清理
clean:
	rm -f $(BINARY)

.PHONY: build preview test test-security clean
