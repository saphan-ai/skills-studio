# The skills image. See image/versions.env for the pinned bases and the image name.
ROOT := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
export ROOT

.PHONY: help image image-check scan push
help: ## List the targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | sed 's/:.*## /\t/'

image: ## Build the skills image for this machine and load it into the local engine
	@sh $(ROOT)/image/scripts/image.sh

image-check: ## Check the image prints the same version and the same verify output as a local build
	@sh $(ROOT)/image/scripts/image-check.sh

scan: ## Scan the local image with Docker Scout; refuses when Scout is missing, fails, or finds critical or high
	@sh $(ROOT)/image/scripts/scan.sh

push: ## Build for linux/amd64 and linux/arm64 once, scan both, then publish those scanned digests (the owner's act)
	@sh $(ROOT)/image/scripts/push.sh
