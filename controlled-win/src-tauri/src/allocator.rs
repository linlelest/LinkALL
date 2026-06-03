// Windows-only：mimalloc 全局 allocator
// 减少内存峰值 ~10-30%，对截屏/编码场景尤其明显
#[cfg(windows)]
use mimalloc::MiMalloc;

#[cfg(windows)]
#[global_allocator]
static GLOBAL: MiMalloc = MiMalloc;

#[cfg(windows)]
pub struct MimallocAlloc;

#[cfg(windows)]
impl MimallocAlloc {
    pub const fn new() -> Self { Self }
}
