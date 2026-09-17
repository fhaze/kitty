import subprocess
import sys

from kitty.fast_data_types import num_users

from .base import BaseTest


class UTMPTest(BaseTest):
    def test_num_users(self):
        if sys.platform == 'win32':
            # who from MSYS2 reads utmp which does not exist on Windows, so just
            # check that the current session is counted
            self.assertGreaterEqual(num_users(), 1)
            return
        # who is the control
        try:
            expected = subprocess.check_output(['who']).decode('utf-8').count('\n')
        except FileNotFoundError:
            self.skipTest('No who executable cannot verify num_users')
        else:
            self.ae(num_users(), expected)
