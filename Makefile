# Makefile (compatibility wrapper for Task)

.PHONY: help
help:
	@task --list

# Proxy all targets to Task
%:
	@task $@
