// 开机自启：Windows 写注册表 Run 键；Linux 写 ~/.config/autostart/*.desktop
#[cfg(windows)]
mod imp {
    use anyhow::Result;
    use auto_launch::AutoLaunchBuilder;
    use once_cell::sync::OnceCell;

    static AL: OnceCell<auto_launch::AutoLaunch> = OnceCell::new();
    fn al() -> &'static auto_launch::AutoLaunch {
        AL.get_or_init(|| {
            let exe = std::env::current_exe().unwrap_or_else(|_| std::path::PathBuf::from("linkall-hosted"));
            AutoLaunchBuilder::new()
                .set_app_name("LinkALL Hosted")
                .set_app_path(exe.to_string_lossy().as_ref())
                .build()
                .expect("auto-launch init")
        })
    }
    pub fn set(on: bool) -> Result<()> { if on { al().enable()? } else { al().disable()? } Ok(()) }
    pub fn get() -> Result<bool> { Ok(al().is_enabled()?) }
}

#[cfg(target_os = "linux")]
mod imp {
    use anyhow::Result;
    use std::fs;
    use std::path::PathBuf;
    fn desktop_file() -> Result<PathBuf> {
        let home = std::env::var("HOME")?;
        let p = PathBuf::from(home).join(".config/autostart/linkall-hosted.desktop");
        if let Some(par) = p.parent() { fs::create_dir_all(par).ok(); }
        Ok(p)
    }
    pub fn set(on: bool) -> Result<()> {
        let p = desktop_file()?;
        if on {
            let exe = std::env::current_exe()?;
            let body = format!("[Desktop Entry]\nType=Application\nName=LinkALL Hosted\nExec={}\nX-GNOME-Autostart-enabled=true\n", exe.display());
            fs::write(p, body)?;
        } else if p.exists() {
            fs::remove_file(p).ok();
        }
        Ok(())
    }
    pub fn get() -> Result<bool> { Ok(desktop_file()?.exists()) }
}

#[cfg(not(any(windows, target_os = "linux")))]
mod imp {
    use anyhow::Result;
    pub fn set(_on: bool) -> Result<()> { Ok(()) }
    pub fn get() -> Result<bool> { Ok(false) }
}

pub use imp::{get, set};
