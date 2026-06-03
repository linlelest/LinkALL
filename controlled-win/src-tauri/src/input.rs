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
    Some(match code {
        "Backspace" => Backspace,
        "Tab" => Tab,
        "Enter" | "NumpadEnter" => Return,
        "Escape" => Escape,
        "Space" => Space,
        "ArrowLeft" => LeftArrow,
        "ArrowRight" => RightArrow,
        "ArrowUp" => UpArrow,
        "ArrowDown" => DownArrow,
        "Shift" | "ShiftLeft" | "ShiftRight" => Shift,
        "Control" | "Ctrl" => Control,
        "Alt" | "AltLeft" | "AltRight" => Alt,
        "Meta" | "Win" => Meta,
        "CapsLock" => CapsLock,
        "Home" => Home,
        "End" => End,
        "PageUp" => PageUp,
        "PageDown" => PageDown,
        "Delete" => Delete,
        "Insert" => Insert,
        "F1" => F1, "F2" => F2, "F3" => F3, "F4" => F4, "F5" => F5, "F6" => F6,
        "F7" => F7, "F8" => F8, "F9" => F9, "F10" => F10, "F11" => F11, "F12" => F12,
        _ => {
            // 单字符
            if code.len() == 1 {
                let ch = code.chars().next().unwrap();
                if ch.is_ascii_alphabetic() || ch.is_ascii_digit() || ch.is_ascii_punctuation() || ch == ' ' {
                    return Some(Key::Unicode(ch));
                }
            }
            return None;
        }
    })
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
