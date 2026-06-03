use anyhow::Result;
use cpal::traits::{DeviceTrait, HostTrait, StreamTrait};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::time::Duration;
use tokio::sync::mpsc;
use webrtc::media::Sample;
use webrtc::track::track_local::track_local_static_sample::TrackLocalStaticSample;

static AUDIO_ACTIVE: AtomicBool = AtomicBool::new(false);
static AUDIO_STOP: AtomicBool = AtomicBool::new(false);

const FRAME_SIZE: usize = 960; // 48kHz * 20ms

pub fn is_running() -> bool {
    AUDIO_ACTIVE.load(Ordering::Acquire)
}

pub fn start(track: Arc<TrackLocalStaticSample>) -> Result<()> {
    if AUDIO_ACTIVE.swap(true, Ordering::AcqRel) {
        return Err(anyhow::anyhow!("audio capture already running"));
    }
    AUDIO_STOP.store(false, Ordering::Release);

    let (encoded_tx, mut encoded_rx) = mpsc::channel::<Vec<u8>>(256);
    let stop = Arc::new(AtomicBool::new(false));
    let stop_clone = stop.clone();

    std::thread::Builder::new()
        .name("audio-capture".into())
        .spawn(move || {
            if let Err(e) = capture_loop(encoded_tx, stop_clone) {
                log::error!("audio capture: {e:?}");
            }
            AUDIO_ACTIVE.store(false, Ordering::Release);
        })?;

    let track_clone = track.clone();
    tokio::spawn(async move {
        while let Some(packet) = encoded_rx.recv().await {
            if AUDIO_STOP.load(Ordering::Acquire) {
                break;
            }
            let sample = Sample {
                data: packet,
                duration: Duration::from_millis(20),
                ..Default::default()
            };
            let _ = track_clone.write_sample(&sample).await;
        }
    });

    Ok(())
}

pub fn stop() {
    AUDIO_STOP.store(true, Ordering::Release);
    for _ in 0..25 {
        if !AUDIO_ACTIVE.load(Ordering::Acquire) {
            break;
        }
        std::thread::sleep(Duration::from_millis(20));
    }
}

fn capture_loop(tx: mpsc::Sender<Vec<u8>>, stop: Arc<AtomicBool>) -> Result<()> {
    let host = cpal::default_host();
    let device = host
        .default_input_device()
        .ok_or_else(|| anyhow::anyhow!("no microphone found"))?;

    let config: cpal::StreamConfig = cpal::StreamConfig {
        channels: 1,
        sample_rate: cpal::SampleRate(48000),
        buffer_size: cpal::BufferSize::Default,
    };

    let mut encoder = opus::Encoder::new(48000, opus::Channels::Mono, opus::Application::Voip)?;
    let _ = encoder.set_bitrate(opus::Bitrate::Bits(64000));
    let mut pcm_buf: Vec<i16> = Vec::with_capacity(FRAME_SIZE);

    let err_fn = |err: cpal::StreamError| log::error!("cpal: {err:?}");
    let tx = tx;
    let stop2 = stop.clone();

    let stream = device.build_input_stream::<f32>(
        &config,
        move |data: &[f32], _: &_| {
            if stop2.load(Ordering::Acquire) {
                return;
            }
            for &s in data {
                pcm_buf.push((s * 32767.0).clamp(-32768.0, 32767.0) as i16);
                if pcm_buf.len() >= FRAME_SIZE {
                    let mut out = vec![0u8; 1275];
                    if let Ok(len) = encoder.encode(&pcm_buf[..FRAME_SIZE], &mut out) {
                        out.truncate(len);
                        let _ = tx.blocking_send(out);
                    }
                    pcm_buf.drain(..FRAME_SIZE);
                }
            }
        },
        err_fn,
        None,
    )?;

    stream.play()?;
    while !stop.load(Ordering::Acquire) {
        std::thread::sleep(Duration::from_millis(100));
    }
    drop(stream);
    Ok(())
}
