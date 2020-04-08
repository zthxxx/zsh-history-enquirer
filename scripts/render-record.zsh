#!/usr/bin/env zsh

# WORKDIR: ../ (project root)

# svg-term
npx svg-term --window --in ./images/zsh-record.cast --out ./images/preview.svg

# replace:font
perl -i -pe 's/"Monaco,/"Menlo,Monaco,/g' ./images/preview.svg

# replace:arrow
perl -i -pe 's/➤/▶/g' ./images/preview.svg
