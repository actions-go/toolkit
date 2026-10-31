package cache

import "os"

// umask is 0 on Windows, which has none.
func umask() os.FileMode { return 0 }
