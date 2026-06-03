// Entry point for Windows
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

fn main() {
    linkall_controlled_win_lib::run();
}
