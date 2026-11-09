# Reuse upstream "make format generate", excluding only doc/link checks.
/^format:/ { sub(/ docs([[:space:]]|$)/, " ") }
/^generate:/ { sub(/ generate-docs([[:space:]]|$)/, " ") }
{
	gsub(/gofmt -s -w \./, "git ls-files -z \047*.go\047 \047:(exclude)vendor/**\047 | xargs -0 -r gofmt -s -w")
	gsub(/\$\(CONTAINER_CLI\) build /, "$(CONTAINER_CLI) build --label org.opencontainers.image.revision=$(BUILD_REVISION) --label org.opencontainers.image.version=$(VERSION) ")
}
/^\tfind \. -name \*\.jsonnet/ {
	$0 = "\tfind . -type f \\( -name \047*.jsonnet\047 -o -name \047*.libsonnet\047 \\) -not -path \047*/vendor/*\047 -print0 | xargs -0 -r $(JSONNETFMT_BINARY) -i"
}
{ sub(/[[:blank:]]+$/, ""); print }
