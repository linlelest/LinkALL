use anyhow::Result;

pub fn write(text: &str) -> Result<()> {
    let mut cb = arboard::Clipboard::new()?;
    cb.set_text(text)?;
    Ok(())
}

pub fn read() -> Result<String> {
    let mut cb = arboard::Clipboard::new()?;
    Ok(cb.get_text()?)
}
