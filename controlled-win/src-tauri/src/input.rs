// 键鼠注入：跨平台
// Windows 走 enigo（基于 Win32 SendInput）
// Linux 走 enigo（基于 X11 / uinput）

use anyhow::Result;
use enigo::{Direction, Enigo, Key, Keyboard, Mouse, Settings};

static ENIGO: once_cell::sync::Lazy<parking_lot::Mutex<Enigo>> = once_cell::sync::Lazy::new(|| {
    parking_lot::Mutex::new(Enigo::new(&Settings::default()).expect("enigo init"))
});

fn map_button(b: i32) -> enigo::Button {
    match b {
        1 => enigo::Button::Middle,
        2 => enigo::Button::Right,
        _ => enigo::Button::Left,
    }
}

fn vk_to_enigo(code: &str) -> Option<Key> {
    use enigo::Key::*;
    if code.len() == 1 {
        let ch = code.chars().next().unwrap();
        if ch.is_ascii_alphabetic() || ch.is_ascii_digit() || ch.is_ascii_punctuation() || ch == ' ' {
            return Some(Key::Unicode(ch));
        }
    }
    match code {
        "Backspace" => Some(Backspace),
        "Tab" => Some(Tab),
        "Enter" | "NumpadEnter" => Some(Return),
        "Escape" => Some(Escape),
        "Space" => Some(Space),
        "ArrowLeft" => Some(LeftArrow),
        "ArrowRight" => Some(RightArrow),
        "ArrowUp" => Some(UpArrow),
        "ArrowDown" => Some(DownArrow),
        "Shift" | "ShiftLeft" | "ShiftRight" => Some(Shift),
        "Control" | "Ctrl" => Some(Control),
        "Alt" | "AltLeft" | "AltRight" => Some(Alt),
        "Meta" | "Win" => Some(Meta),
        "CapsLock" => Some(CapsLock),
        "Home" => Some(Home),
        "End" => Some(End),
        "PageUp" => Some(PageUp),
        "PageDown" => Some(PageDown),
        "Delete" => Some(Delete),
        "Insert" => Some(Insert),
        "F1" => Some(F1), "F2" => Some(F2), "F3" => Some(F3), "F4" => Some(F4), "F5" => Some(F5), "F6" => Some(F6),
        "F7" => Some(F7), "F8" => Some(F8), "F9" => Some(F9), "F10" => Some(F10), "F11" => Some(F11), "F12" => Some(F12),
        _ => Option::<Key>::None,
    }
}

pub fn send_mouse(x_pct: f64, y_pct: f64, button: i32, down: bool) -> Result<()> {
    // 获取屏幕尺寸，把百分比转为绝对像素
    let (sw, sh) = crate::screen::screen_size().unwrap_or((1920, 1080));
    let x = (x_pct / 100.0 * sw as f64) as i32;
    let y = (y_pct / 100.0 * sh as f64) as i32;
    let mut e = ENIGO.lock();
    e.move_mouse(x, y, enigo::Coordinate::Abs)?;
    let dir = if down { Direction::Press } else { Direction::Release };
    e.button(map_button(button), dir)?;
    Ok(())
}

pub fn send_key(code: &str, down: bool) -> Result<()> {
    if let Some(k) = vk_to_enigo(code) {
        let dir = if down { Direction::Press } else { Direction::Release };
        let mut e = ENIGO.lock();
        e.key(k, dir)?;
    }
    Ok(())
}

pub fn send_wheel(dx: i32, dy: i32) -> Result<()> {
    let mut e = ENIGO.lock();
    e.scroll(dx, enigo::Axis::Horizontal)?;
    e.scroll(dy, enigo::Axis::Vertical)?;
    Ok(())
}

pub fn send_text(t: &str) -> Result<()> {
    let mut e = ENIGO.lock();
    e.text(t)?;
    Ok(())
}

pub fn click(button: i32, x: f32, y: f32, down: bool) -> Result<()> {
    let mut e = ENIGO.lock();
    e.move_mouse(x as i32, y as i32, enigo::Coordinate::Abs)?;
    let dir = if down { Direction::Press } else { Direction::Release };
    e.button(map_button(button), dir)?;
    Ok(())
}

pub fn wheel(dy: i32) -> Result<()> {
    send_wheel(0, dy)
}

pub fn key(code: i32, down: bool) -> Result<()> {
    let dir = if down { Direction::Press } else { Direction::Release };
    let mut e = ENIGO.lock();
    let name = format!("VK_{code}");
    if let Some(k) = vk_to_enigo(&name) {
        e.key(k, dir)?;
    }
    Ok(())
}
