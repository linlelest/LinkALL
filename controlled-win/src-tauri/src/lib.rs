// LinkALL Hosted (controlled) end for Windows
// 入口模块，负责启动 Tauri、托盘、信令、截屏、键鼠注入、隐私屏等子模块

// Windows 端使用 mimalloc 替换默认 allocator：减少内存峰值 ~10-30%
// 在 Linux/macOS 编译时该模块不引入
#[cfg(windows)]
mod allocator;
#[cfg(windows)]
use crate::allocator::MimallocAlloc;

mod config;
mod db;
mod server_api;
mod signaling;
mod webrtc_host;
mod h264;
mod screen;
mod input;
mod privacy;
mod autostart;
mod secure_store;
mod state;
mod logger;
mod hardware;
mod clipboard;
mod recording;
mod toolbar;
mod audio;

use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use parking_lot::Mutex;

pub static RESTART_PENDING: AtomicBool = AtomicBool::new(false);

pub fn set_restart_pending() {
    RESTART_PENDING.store(true, Ordering::Release);
}

pub fn consume_restart_pending() -> bool {
    RESTART_PENDING.swap(false, Ordering::AcqRel)
}
use tauri::{
    menu::{Menu, MenuItem},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    Emitter, Manager, WindowEvent,
};

use crate::state::AppState;

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    env_logger::Builder::from_env(env_logger::Env::default().default_filter_or("info"))
        .format_timestamp_secs()
        .init();

    let state = Arc::new(AppState::new());

    tauri::Builder::default()
        .plugin(tauri_plugin_os::init())
        .manage(state.clone())
        .setup(move |app| {
            // 托盘
            let show = MenuItem::with_id(app, "show", "显示主窗口", true, None::<&str>)?;
            let toggle = MenuItem::with_id(app, "toggle", "启动/停止服务", true, None::<&str>)?;
            let quit = MenuItem::with_id(app, "quit", "退出", true, None::<&str>)?;
            let menu = Menu::with_items(app, &[&show, &toggle, &quit])?;
            let _tray = TrayIconBuilder::with_id("main-tray")
                .icon(app.default_window_icon().unwrap().clone())
                .menu(&menu)
                .show_menu_on_left_click(false)
                .on_menu_event(|app, event| match event.id.as_ref() {
                    "show" => { if let Some(w) = app.get_webview_window("main") { let _ = w.show(); let _ = w.set_focus(); } }
                    "toggle" => { let st = app.state::<Arc<AppState>>().inner().clone(); tokio::spawn(async move { state::toggle_service(st).await; }); }
                    "quit" => { app.exit(0); }
                    _ => {}
                })
                .on_tray_icon_event(|tray, event| {
                    if let TrayIconEvent::Click { button: MouseButton::Left, button_state: MouseButtonState::Up, .. } = event {
                        let app = tray.app_handle();
                        if let Some(w) = app.get_webview_window("main") { let _ = w.show(); let _ = w.set_focus(); }
                    }
                })
                .build(app)?;

            // 监听主窗口关闭：最小化到托盘
            if let Some(window) = app.get_webview_window("main") {
                let app_handle = app.handle().clone();
                window.on_window_event(move |ev| {
                    if let WindowEvent::CloseRequested { api, .. } = ev {
                        if let Some(w) = app_handle.get_webview_window("main") {
                            let _ = w.hide();
                        }
                        api.prevent_close();
                    }
                });
            }

            // 启动后台服务
            let st = state.clone();
            let app_handle = app.handle().clone();
            tauri::async_runtime::spawn(async move {
                if let Err(e) = state::start_service(st, app_handle).await {
                    log::error!("start service failed: {e:?}");
                }
            });
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            cmd::get_server,
            cmd::set_server,
            cmd::get_locale,
            cmd::set_locale,
            cmd::register_device,
            cmd::login_device,
            cmd::get_device,
            cmd::update_flags,
            cmd::reset_code,
            cmd::logout_device,
            cmd::get_status,
            cmd::start_service,
            cmd::stop_service,
            cmd::set_autostart,
            cmd::get_autostart,
            cmd::quit_app,
            cmd::show_main,
            cmd::respond_request,
            cmd::get_ice_servers,
            cmd::list_displays,
            cmd::select_display,
            cmd::get_selected_display,
            cmd::report_crash,
            cmd::upload_logs,
            cmd::log_write,
            cmd::secure_store_mode,
            cmd::export_secure_key,
            cmd::import_secure_key,
            cmd::recover_from_machine_backup,
            cmd::show_toolbar,
            cmd::hide_toolbar,
            cmd::end_session,
            cmd::start_recording,
            cmd::stop_recording,
            hardware::get_hw_capability,
            hardware::re_probe_hw,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}

mod cmd;
