SHELL=/usr/bin/env bash -o pipefail

PRJ_DIR = $(shell git rev-parse --show-toplevel)
OLM_DIR = $(PRJ_DIR)/rhobs/olm

CONTAINER_RUNTIME := $(shell command -v podman 2> /dev/null || echo docker)

IMAGE_REPO ?= "local-registry"
# IMAGE_BASE defines the registry/namespace and part of the image name
# This variable is used to construct full image tags for bundle and catalog images.
IMAGE_BASE = $(IMAGE_REPO)/obo

VERSION ?= $(shell cat $(PRJ_DIR)/VERSION)
RELEASE_SHA ?= $(shell git rev-parse downstream/main)
OPERATOR_IMG = $(IMAGE_BASE)-prometheus-operator:v$(VERSION)
CONFIG_RELOADER_IMG = $(IMAGE_BASE)-prometheus-config-reloader:v$(VERSION)
ADMISSION_WEBHOOK_IMG = $(IMAGE_BASE)-admission-webhook:v$(VERSION)
OPERATOR_NAME = obo-prometheus-operator

# tools
TOOLS_DIR = $(PRJ_DIR)/tmp/bin

KUSTOMIZE=$(TOOLS_DIR)/kustomize
KUSTOMIZE_VERSION= v5.0.1

OPERATOR_SDK = $(TOOLS_DIR)/operator-sdk
OPERATOR_SDK_VERSION = v1.28.1

OPM=$(TOOLS_DIR)/opm
OPM_VERSION = v1.26.5

$(TOOLS_DIR):
	@mkdir -p $(TOOLS_DIR)

.PHONY: kustomize
$(KUSTOMIZE) kustomize: $(TOOLS_DIR)
	@{ \
		set -ex ;\
		[[ -f $(KUSTOMIZE) ]] && exit 0 ;\
		GOBIN=$(TOOLS_DIR) go install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION) ;\
	}

.PHONY: operator-sdk
$(OPERATOR_SDK) operator-sdk: $(TOOLS_DIR)
	@{ \
		set -ex ;\
		[[ -f $(OPERATOR_SDK) ]] && \
		[[ "$$( $(OPERATOR_SDK) version | awk '{print $$3}' | tr -d ,)" == \"$(OPERATOR_SDK_VERSION)\" ]] && { \
			echo "operator-sdk up to date" ;\
			exit 0 ;\
		} ;\
		OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
		curl -sSLo $(OPERATOR_SDK) https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$${OS}_$${ARCH} ;\
		chmod +x $(OPERATOR_SDK) ;\
	}

.PHONY: opm
$(OPM) opm: $(TOOLS_DIR)
	@{ \
		[[ -f $(OPM) ]] && exit 0 ;\
		set -ex ;\
		OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
		curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/$(OPM_VERSION)/$${OS}-$${ARCH}-opm ;\
		chmod +x $(OPM) ;\
	}

tools: kustomize operator-sdk opm


## OLM - Bundle

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_BASE)-prometheus-operator-bundle:v$(VERSION)

# CHANNELS define the bundle channels used in the bundle.
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
CHANNELS ?= stable
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# To re-generate a bundle for any other default channel without changing the default setup, use:
# - DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
DEFAULT_CHANNEL ?= stable

ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)


# copy-examples copies examples required to create the olm bundle to
# appropriate component's dir
copy-examples:
	@{ \
		set -x ;\
		mkdir -p \
			$(OLM_DIR)/manifests/operator/example  \
			$(OLM_DIR)/manifests/admission-webhook/example  \
			$(OLM_DIR)/manifests/crds/example ;\
			\
		cp -r $(PRJ_DIR)/example/stripped-down-crds/ $(OLM_DIR)/manifests/crds/example/ ;\
		cp -r $(PRJ_DIR)/example/rbac/ $(OLM_DIR)/manifests/operator/example/ ;\
		cp -r $(PRJ_DIR)/example/admission-webhook/ $(OLM_DIR)/manifests/admission-webhook/example/ ;\
	}

clean-examples:
	@{ \
		set -x ;\
		cd $(OLM_DIR) ;\
		rm -rf \
			$(PRJ_DIR)/manifests/operator/example \
			$(PRJ_DIR)/manifests/admission-webhook/example \
			$(PRJ_DIR)/manifests/crds/example ;\
	}

.PHONY: bundle
bundle: copy-examples
	@{ \
		set -x ;\
		cd $(OLM_DIR) ;\
		\
		$(KUSTOMIZE) build manifests | \
			sed \
				-e "s|<IMG_PROMETHEUS_OPERATOR>|$(OPERATOR_IMG)|g" \
				-e "s|<IMG_PROMETHEUS_CONFIG_RELOADER>|$(CONFIG_RELOADER_IMG)|g" \
				-e "s|<IMG_ADMISSION_WEBHOOK>|$(ADMISSION_WEBHOOK_IMG)|g" | \
			tee $(PRJ_DIR)/tmp/pre-bundle.yaml |  \
			$(OPERATOR_SDK) generate bundle \
				--overwrite \
				--version $(VERSION) \
				--package=$(OPERATOR_NAME) \
				$(BUNDLE_METADATA_OPTS) ;\
		\
		$(OPERATOR_SDK) bundle validate ./bundle ;\
	}

.PHONY: bundle-image
bundle-image: bundle ## Build the bundle image.
	$(CONTAINER_RUNTIME) build -f $(OLM_DIR)/bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Build the bundle image.
	$(CONTAINER_RUNTIME) push $(PUSH_OPTIONS) $(BUNDLE_IMG)

# Catalog

OPERATOR_BUNDLE=$(OPERATOR_NAME).v$(VERSION)

# The image tag given to the resulting catalog image
CATALOG_IMG_BASE ?= $(IMAGE_BASE)-prometheus-operator-catalog
CATALOG_IMG ?= $(CATALOG_IMG_BASE):v$(VERSION)

# The tag is used as latest since it allows a CatalogSubscription to point to
# a single image which keeps updating there by allowing auto upgrades
CATALOG_IMG_LATEST ?= $(CATALOG_IMG_BASE):latest
CATALOG_INDEX_DIR = $(OLM_DIR)/catalog/config
CATALOG_INDEX_FILE = $(CATALOG_INDEX_DIR)/index.yaml
CATALOG_INDEX_CONTAINER_FILE = $(OLM_DIR)/catalog/index.Dockerfile

# Build a catalog image by adding bundle images to an empty catalog using the
# operator package manager tool, 'opm'.
.PHONY: catalog-image
catalog-image: $(OPM)
	# add bundle to catalog index only if it doesn't exist
	grep -q $(BUNDLE_IMG) $(CATALOG_INDEX_FILE) || { \
		$(OPM) render $(BUNDLE_IMG) --output=yaml >> $(CATALOG_INDEX_FILE) ;\
		./catalog/update-channels.sh $(CATALOG_INDEX_FILE) $(CHANNELS) $(OPERATOR_BUNDLE) ;\
	}
	$(OPM) validate $(CATALOG_INDEX_DIR)

	$(CONTAINER_RUNTIME) build  \
		-f $(CATALOG_INDEX_CONTAINER_FILE)  -t $(CATALOG_IMG)

	# tag the catalog img:version as latest so that continious release
	# is possible by refering to latest tag instead of a version
	$(CONTAINER_RUNTIME) tag $(CATALOG_IMG) $(CATALOG_IMG_LATEST)

# NOTE: This is required to enable continuous deployment to
# staging/integration environments via app-interface (OSD-13603)
#
# The git short-hash of the most recent commit in the main branch.
# This will be used to associate the catalog image with the operator code that
# was used to build the imate.
CATALOG_IMG_SHA = $(CATALOG_IMG_BASE):$(shell git rev-parse --short=8 $(RELEASE_SHA))

# NOTE: This target ensures that the catalog image points to the
# commit in the main branch that was used to build the catalog image
# In a prior version we used the commit on the olm-catalog branch to tag this.
.PHONY: catalog-tag-sha
catalog-tag-sha: ## Push a catalog image.
	$(CONTAINER_RUNTIME) tag $(CATALOG_IMG) $(CATALOG_IMG_SHA)

# Push the catalog image.
.PHONY: catalog-push
catalog-push: catalog-tag-sha ## Push a catalog image.
	$(CONTAINER_RUNTIME) push $(PUSH_OPTIONS) $(CATALOG_IMG)
	$(CONTAINER_RUNTIME) push $(PUSH_OPTIONS) $(CATALOG_IMG_LATEST)
	$(CONTAINER_RUNTIME) push $(PUSH_OPTIONS) $(CATALOG_IMG_SHA)
