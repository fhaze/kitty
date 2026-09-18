#!/usr/bin/env python
# License: GPLv3 Copyright: 2026, Kovid Goyal <kovid at kovidgoyal.net>

import os
import shutil
import stat
import subprocess
import tempfile
from unittest.mock import patch

from kitty.constants import is_windows, kitty_base_dir
from kitty.wsl import decode_wsl_output, install_script, main, parse_distro_list, wsl_command

from .base import BaseTest


class WSLTest(BaseTest):
    def test_decode_wsl_output(self):
        self.ae(decode_wsl_output('Ubuntu\r\nDebian\r\n'.encode('utf-16-le')), 'Ubuntu\r\nDebian\r\n')
        self.ae(decode_wsl_output('\ufeffUbuntu\r\n'.encode('utf-16-le')), 'Ubuntu\r\n')
        self.ae(decode_wsl_output(b'Ubuntu\nDebian\n'), 'Ubuntu\nDebian\n')
        self.ae(decode_wsl_output(b''), '')

    def test_parse_distro_list(self):
        self.ae(parse_distro_list('Ubuntu\r\n\r\ndocker-desktop\r\nDebian \r\nDocker-Desktop-Data\r\n'), ['Ubuntu', 'Debian'])
        self.ae(parse_distro_list(''), [])

    def test_wsl_command(self):
        cmd = wsl_command('Ubuntu', ('--uninstall',))
        self.assertTrue(cmd[0].lower().endswith('wsl.exe'))
        self.ae(cmd[1:], ['--distribution', 'Ubuntu', '--exec', 'sh', '-s', '--', '--uninstall'])

    def test_main(self):
        calls = []

        def fake_run(distro, uninstall=False, update_path=True):
            calls.append((distro, uninstall, update_path))
            return distro != 'broken'

        with patch('kitty.wsl.run_in_distro', fake_run), patch('kitty.wsl.is_windows', True), patch('kitty.wsl.os.path.exists', lambda p: True):
            with patch('kitty.wsl.list_distros', lambda: ['Ubuntu', 'Debian']):
                main(['wsl-setup'])
                self.ae(calls, [('Ubuntu', False, True), ('Debian', False, True)])
                del calls[:]
                main(['wsl-setup', '--uninstall', '--no-path', 'Alpine'])
                self.ae(calls, [('Alpine', True, False)])
                del calls[:]
            with patch('kitty.wsl.list_distros', lambda: []):
                main(['wsl-setup'])
                self.ae(calls, [])
            with self.assertRaises(SystemExit):
                main(['wsl-setup', 'broken'])
            with self.assertRaises(SystemExit):
                main(['wsl-setup', '--bad-option'])

    def test_install_script(self):
        # Runs windows/wsl/install-kitten.sh the way kitty +wsl-setup does inside a distribution
        sh = shutil.which('sh')
        if is_windows or not sh:
            self.skipTest('needs a POSIX sh')
        self.assertTrue(os.path.exists(install_script), install_script)
        with open(install_script, 'rb') as f:
            script = f.read()
        tdir = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, tdir)
        home = os.path.join(tdir, 'home')
        src = os.path.join(tdir, 'src')
        fake_bin = os.path.join(tdir, 'fakebin')
        for x in (home, src, fake_bin):
            os.makedirs(x)
        machine = os.uname().machine
        arch = {'x86_64': 'amd64', 'amd64': 'amd64', 'aarch64': 'arm64', 'arm64': 'arm64'}.get(machine)
        if not arch:
            self.skipTest(f'unsupported machine: {machine}')

        def write_executable(path: str, contents: str) -> None:
            with open(path, 'w') as f:
                f.write(contents)
            os.chmod(path, os.stat(path).st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)

        write_executable(os.path.join(src, f'kitten-linux-{arch}'), '#!/bin/sh\necho kitten 0.0.0-test\n')
        bin_dir = os.path.join(home, '.local', 'bin')
        kitten = os.path.join(bin_dir, 'kitten')

        def run(*args: str, login_shell: str = '/bin/bash') -> str:
            # the script consults getent for the login shell of the user
            write_executable(os.path.join(fake_bin, 'getent'), f'#!/bin/sh\necho "user:x:1000:1000::{home}:{login_shell}"\n')
            env = dict(os.environ, HOME=home, PATH=f'{fake_bin}{os.pathsep}{os.environ["PATH"]}')
            env.pop('ZDOTDIR', None)
            env.pop('XDG_CONFIG_HOME', None)
            cp = subprocess.run([sh, '-s', '--', *args], input=script, capture_output=True, env=env, cwd=tdir)
            self.ae(cp.returncode, 0, cp.stderr.decode())
            return cp.stdout.decode()

        def rc(name: str) -> str:
            with open(os.path.join(home, name)) as f:
                return f.read()

        with open(os.path.join(home, '.bashrc'), 'w') as f:
            f.write('export FOO=1\n')
        out = run(src)
        self.assertIn(f'Installed kitten 0.0.0-test to {kitten}', out)
        self.assertIn('.bashrc', out)
        self.assertTrue(os.access(kitten, os.X_OK))
        self.assertIn(f'export PATH="{bin_dir}:$PATH"', rc('.bashrc'))
        self.ae(rc('.bashrc').count('kitty-wsl-kitten'), 2)
        # idempotent
        run(src)
        self.ae(rc('.bashrc').count('kitty-wsl-kitten'), 2)
        cp = subprocess.run([sh, '-c', '. "$HOME/.bashrc"; command -v kitten'], capture_output=True, env=dict(os.environ, HOME=home))
        self.ae(cp.stdout.decode().strip(), kitten)

        run(src, login_shell='/usr/bin/zsh')
        self.assertIn(f'export PATH="{bin_dir}:$PATH"', rc('.zshrc'))
        run(src, login_shell='/usr/bin/fish')
        self.assertIn(f'set -gx PATH "{bin_dir}" $PATH', rc('.config/fish/conf.d/kitty-wsl-kitten.fish'))
        run(src, login_shell='/usr/bin/tcsh')
        self.assertIn(f'setenv PATH "{bin_dir}:${{PATH}}"', rc('.tcshrc'))
        run(src, login_shell='/bin/dash')
        self.assertIn(f'export PATH="{bin_dir}:$PATH"', rc('.profile'))
        self.assertFalse(os.path.exists(os.path.join(home, '.bash_profile')))

        out = run('--uninstall')
        self.assertIn(f'Removed {kitten}', out)
        self.assertFalse(os.path.exists(kitten))
        self.ae(rc('.bashrc'), 'export FOO=1\n')
        for x in ('.zshrc', '.tcshrc', '.profile', '.config/fish/conf.d/kitty-wsl-kitten.fish'):
            self.assertFalse(os.path.exists(os.path.join(home, x)), x)
        # uninstalling again is a no-op
        run('--uninstall')

    def test_install_script_shipped(self):
        self.ae(install_script, os.path.join(kitty_base_dir, 'windows', 'wsl', 'install-kitten.sh'))
        self.assertTrue(os.path.exists(install_script), install_script)
