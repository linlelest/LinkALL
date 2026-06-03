// 浮动工具栏窗口（alwaysOnTop / draggable / 小尺寸）
//  - show_toolbar / hide_toolbar / set_toolbar_session
//  - 由会话生命周期调用：start_service → show，session end → hide
use tauri::{AppHandle, Manager, WebviewWindowBuilder, WebviewUrl};

pub const TOOLBAR_LABEL: &str = "toolbar";

/// 创建工具栏窗口（如果已存在则复用）
pub fn ensure_toolbar(app: &AppHandle) -> tauri::Result<()> {
    if app.get_webview_window(TOOLBAR_LABEL).is_some() {
        return Ok(());
    }
    let url = WebviewUrl::App("toolbar.html".into());
    WebviewWindowBuilder::new(app, TOOLBAR_LABEL, url)
        .title("LinkALL Toolbar")
        .inner_size(360.0, 56.0)
        .min_inner_size(280.0, 56.0)
        .max_inner_size(600.0, 56.0)
        .resizable(false)
        .decorations(false)
        .transparent(true)
        .always_on_top(true)
        .skip_taskbar(true)
        .focused(false)
        .visible(false)
        .build()?;
    Ok(())
}

pub fn show_toolbar(app: &AppHandle) {
    if let Err(e) = ensure_toolbar(app) {
        log::warn!("[toolbar] ensure failed: {e:?}");
        return;
    }
    if let Some(w) = app.get_webview_window(TOOLBAR_LABEL) {
        let _ = w.show();
        // 不抢焦点（focus: false + set_focus skip）
    }
}

pub fn hide_toolbar(app: &AppHandle) {
    if let Some(w) = app.get_webview_window(TOOLBAR_LABEL) {
        let _ = w.hide();
    }
}

pub fn destroy_toolbar(app: &AppHandle) {
    if let Some(w) = app.get_webview_window(TOOLBAR_LABEL) {
        let _ = w.close();
    }
}
