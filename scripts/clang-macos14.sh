#!/bin/sh

exec clang "$@" -mmacosx-version-min=10.14
