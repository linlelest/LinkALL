// 防窥屏（黑屏覆盖）：在被控端主显示器全屏叠加一个不透明黑色窗口
// Windows: 全屏 WS_EX_TOPMOST 透明（其实用纯黑更直接）窗口
// 这里用 parked thread 维护一个常驻窗口

#[cfg(windows)]
mod imp {
    use parking_lot::Mutex;
    use std::sync::Arc;
    use windows::Win32::Foundation::{HWND, LPARAM, WPARAM};
    use windows::Win32::Graphics::Gdi::{GetDC, HBRUSH};
    use windows::Win32::UI::WindowsAndMessaging::{
        CreateWindowExW, DefWindowProcW, DispatchMessageW, GetSystemMetrics, RegisterClassW,
        ShowWindow, TranslateMessage, MSG, SM_CXSCREEN, SM_CYSCREEN, SW_SHOW,
        WINDOW_EX_STYLE, WNDCLASSW, CS_HREDRAW, CS_VREDRAW, WS_EX_TOPMOST, WS_POPUP,
    };

    static RUNNING: Mutex<bool> = Mutex::new(false);

    pub fn set(on: bool) -> anyhow::Result<()> {
        if on {
            if *RUNNING.lock() { return Ok(()); }
            *RUNNING.lock() = true;
            std::thread::spawn(|| unsafe {
                let class_name: Vec<u16> = "LinkALL_Privacy\0".encode_utf16().collect();
                let title: Vec<u16> = "LinkALL Privacy\0".encode_utf16().collect();
                let wc = WNDCLASSW {
                    style: CS_HREDRAW | CS_VREDRAW,
                    lpfnWndProc: Some(wnd_proc),
                    lpszClassName: windows::core::PCWSTR(class_name.as_ptr()),
                    ..Default::default()
                };
                RegisterClassW(&wc);
                let w = GetSystemMetrics(SM_CXSCREEN);
                let h = GetSystemMetrics(SM_CYSCREEN);
                let hwnd = CreateWindowExW(
                    WS_EX_TOPMOST,
                    windows::core::PCWSTR(class_name.as_ptr()),
                    windows::core::PCWSTR(title.as_ptr()),
                    WS_POPUP,
                    0, 0, w, h,
                    HWND(std::ptr::null_mut()),
                    None,
                    None,
                    None,
                );
                if !hwnd.is_err() {
                    ShowWindow(hwnd.unwrap(), SW_SHOW);
                    // 屏蔽点击
                    loop {
                        let mut msg = MSG::default();
                        let r = windows::Win32::UI::WindowsAndMessaging::GetMessageW(&mut msg, HWND(std::ptr::null_mut()), 0, 0);
                        if r.0 == 0 { break; }
                        TranslateMessage(&msg);
                        DispatchMessageW(&msg);
                        if !*RUNNING.lock() { break; }
                    }
                }
                *RUNNING.lock() = false;
            });
        } else {
            *RUNNING.lock() = false;
            // 窗口在下一次 GetMessageW 循环中退出
        }
        Ok(())
    }

    unsafe extern "system" fn wnd_proc(hwnd: HWND, msg: u32, wparam: WPARAM, lparam: LPARAM) -> windows::Win32::Foundation::LRESULT {
        use windows::Win32::UI::WindowsAndMessaging::WM_DESTROY;
        if msg == WM_DESTROY {
            windows::Win32::UI::WindowsAndMessaging::PostQuitMessage(0);
            return windows::Win32::Foundation::LRESULT(0);
        }
        DefWindowProcW(hwnd, msg, wparam, lparam)
    }
}

#[cfg(not(windows))]
mod imp {
    pub fn set(_on: bool) -> anyhow::Result<()> { Ok(()) }
}

pub use imp::set;
