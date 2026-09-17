#!/usr/bin/env python
# License: GPL v3 Copyright: 2026, Kovid Goyal <kovid at kovidgoyal.net>

import os
import shlex

from .base import BaseTest


class TestChooseFiles(BaseTest):
    def test_format_selection_for_paste(self) -> None:
        from kittens.choose_files.main import format_selection_for_paste

        paths = ['/work/simple', '/work/a path', '/work/a;command', '/work/*.txt', '/work/~root', '/work/a>output', "/work/it's"]
        if os.name != 'nt':  # backslash is a path separator on Windows
            paths.append(r'/work/a\b')
        text = format_selection_for_paste(paths, '/work', at_prompt=True)
        self.assertEqual(shlex.split(text), [path.removeprefix('/work/') for path in paths])
        self.assertEqual(format_selection_for_paste(paths[:2], '/work', at_prompt=False), 'simple\na path')
