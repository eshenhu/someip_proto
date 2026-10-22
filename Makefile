BUF ?= buf

.PHONY: \
	bazel-test \
	buf-breaking \
	buf-format \
	buf-format-check \
	buf-lint \
	check \
	fds-smoke \
	go-format-check \
	go-fuzz-smoke \
	go-generate \
	go-generate-check \
	go-test \
	go-vet \
	lock-check \
	phase4-artifact-check \
	release-package

check: buf-format-check buf-lint buf-breaking go-generate-check bazel-test go-format-check go-test go-vet go-fuzz-smoke fds-smoke lock-check phase4-artifact-check

buf-format:
	$(BUF) format -w proto

buf-format-check:
	$(BUF) format --diff proto

buf-lint:
	$(BUF) lint proto

buf-breaking:
	bash ./scripts/check-buf-breaking.sh $(BUF)

bazel-test:
	bazel test //...

fds-smoke:
	bazel test //bazel/rules/testdata:descriptor_smoke_test

go-format-check:
	bash ./scripts/check-go.sh format

go-fuzz-smoke:
	bash ./scripts/check-go.sh fuzz

go-generate:
	$(BUF) generate proto --template buf.gen.yaml

go-generate-check:
	@$(MAKE) --no-print-directory go-generate
	@if ! git diff --quiet -- gen/go || [ -n "$$(git ls-files --others --exclude-standard -- gen/go)" ]; then \
		echo "Generated Go protobuf bindings are out of date. Run 'make go-generate'." >&2; \
		exit 1; \
	fi

go-test:
	bash ./scripts/check-go.sh test

go-vet:
	bash ./scripts/check-go.sh vet

lock-check:
	bash ./scripts/check-locks.sh $(BUF)

phase4-artifact-check:
	bash ./scripts/check-phase4-artifacts.sh

release-package:
	@test -n "$(RELEASE_VERSION)" || { echo "RELEASE_VERSION must be set, for example RELEASE_VERSION=v0.1.0" >&2; exit 2; }
	SOURCE_REVISION="$(SOURCE_REVISION)" OUTPUT_DIR="$(OUTPUT_DIR)" bash ./scripts/release-package.sh "$(RELEASE_VERSION)"
