#!/usr/bin/env python
# License: GPLv3 Copyright: 2025, Kovid Goyal <kovid at kovidgoyal.net>


import os
import select
import shutil
import signal
import subprocess
import sys
import tempfile
import time
from concurrent.futures import ThreadPoolExecutor

from kitty.constants import kitten_exe, kitty_exe
from kitty.shm import SharedMemory

from .base import BaseTest


class Atexit(BaseTest):
    def setUp(self):
        self.tdir = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tdir)

    def test_go_atexit(self):
        cp = subprocess.run([kitten_exe(), '__atexit__', 'test'], cwd=self.tdir)
        self.ae(cp.returncode, 0)
        self.assertFalse(os.listdir(self.tdir))

    def test_atexit(self):

        def r(action='close'):
            p = subprocess.Popen(
                [
                    kitty_exe(),
                    '+runpy',
                    f"""\
import subprocess
p = subprocess.Popen(['{kitten_exe()}', '__atexit__'])
print(p.pid, flush=True)
raise SystemExit(p.wait())
""",
                ],
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
            )
            readers = [p.stdout.fileno()]

            if sys.platform == 'win32':
                # select() does not work on pipes on Windows
                executor = self.enterContext(ThreadPoolExecutor(max_workers=1))

                def read():
                    return executor.submit(p.stdout.readline).result(timeout=10).rstrip().decode()

                def wait_for_readable():
                    pass
            else:

                def read():
                    r, _, _ = select.select(readers, [], [], 10)
                    if not r:
                        raise TimeoutError('Timed out waiting for read from child')
                    return p.stdout.readline().rstrip().decode()

                def wait_for_readable():
                    select.select(readers, [], [], 10)

            atexit_pid = int(read())
            for i in range(2):
                with open(os.path.join(self.tdir, str(i)), 'w') as f:
                    p.stdin.write(f'unlink {f.name}\n'.encode())
                    p.stdin.flush()
                wait_for_readable()
                self.ae(read(), str(i + 1))
            sdir = os.path.join(self.tdir, 'd')
            os.mkdir(sdir)
            p.stdin.write(f'rmtree {sdir}\n'.encode())
            p.stdin.flush()
            open(os.path.join(sdir, 'f'), 'w').close()
            wait_for_readable()
            self.ae(read(), str(i + 2))
            shm = SharedMemory(size=64)
            shm.write(b'1' * 64)
            shm.flush()
            p.stdin.write(f'shm_unlink {shm.name}\n'.encode())
            p.stdin.flush()
            self.ae(read(), str(i + 3))

            self.assertTrue(os.listdir(self.tdir))
            shm2 = SharedMemory(shm.name)
            self.ae(shm2.read()[:64], b'1' * 64)

            # Ensure child is ignoring signals
            if sys.platform != 'win32':  # os.kill() on Windows just terminates the process
                os.kill(atexit_pid, signal.SIGINT)
                os.kill(atexit_pid, signal.SIGTERM)
            if action == 'close':
                p.stdin.close()
            elif action == 'terminate':
                p.terminate()
            else:
                p.kill()
            p.wait(10)
            if action != 'close':
                p.stdin.close()
            wait_for_readable()
            self.assertFalse(read())
            p.stdout.close()
            if sys.platform == 'win32':
                # the atexit process is not our child, so poll for it to finish cleanup
                from kitty.fast_data_types import monotonic

                st = monotonic()
                while os.listdir(self.tdir) and monotonic() - st < 10:
                    time.sleep(0.01)
            else:
                try:
                    os.waitpid(atexit_pid, 0)
                except ChildProcessError:
                    pass
            self.assertFalse(os.listdir(self.tdir))
            self.assertRaises(FileNotFoundError, lambda: SharedMemory(shm.name))

        r('close')
        r('terminate')
        r('kill')
