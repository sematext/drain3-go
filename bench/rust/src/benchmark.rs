use drain3::{Config, Drain};
use serde::Serialize;
use std::collections::HashMap;
use std::fs::File;
use std::io::{self, BufReader, Read};
use std::time::Instant;

const SEED: u64 = 14695981039346656037;
const PRIME: u64 = 1099511628211;
fn digest(h: u64, partition: u32, id: u64) -> u64 {
    ((h ^ partition as u64).wrapping_mul(PRIME) ^ id).wrapping_mul(PRIME)
}
#[derive(Serialize)]
struct Cluster {
    partition: u32,
    id: u64,
    size: u64,
    template: String,
}
#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct ResultRow {
    parser: &'static str,
    rows: usize,
    train_ns: u64,
    match_ns: u64,
    train_digest: u64,
    match_digest: u64,
    unmatched: usize,
    clusters: Vec<Cluster>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    train_ids: Vec<u64>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    match_ids: Vec<u64>,
}
fn frames(path: &str, mut f: impl FnMut(u32, &str)) -> io::Result<()> {
    let mut input = BufReader::with_capacity(1 << 20, File::open(path)?);
    let mut header = [0u8; 8];
    let mut body = Vec::new();
    loop {
        match input.read(&mut header[..1])? {
            0 => return Ok(()),
            _ => input.read_exact(&mut header[1..])?,
        }
        let partition = u32::from_le_bytes(header[..4].try_into().unwrap());
        let size = u32::from_le_bytes(header[4..].try_into().unwrap()) as usize;
        if size > 1 << 20 {
            return Err(io::Error::new(
                io::ErrorKind::InvalidData,
                "frame exceeds 1 MiB",
            ));
        }
        body.resize(size, 0);
        input.read_exact(&mut body)?;
        let line = std::str::from_utf8(&body)
            .map_err(|e| io::Error::new(io::ErrorKind::InvalidData, e))?;
        f(partition, line);
    }
}
pub(crate) fn run() -> Result<(), Box<dyn std::error::Error>> {
    let mut args = std::env::args().skip(2);
    let (mut path, mut numeric, mut trace) = (None, false, false);
    while let Some(arg) = args.next() {
        match arg.as_str() {
            "--input" => path = args.next(),
            "--numeric" => numeric = true,
            "--trace" => trace = true,
            _ => return Err(format!("unknown argument {arg}").into()),
        }
    }
    let path = path.ok_or("--input is required")?;
    let mut models: HashMap<u32, Drain> = HashMap::new();
    let mut out = ResultRow {
        parser: "rust",
        rows: 0,
        train_ns: 0,
        match_ns: 0,
        train_digest: SEED,
        match_digest: SEED,
        unmatched: 0,
        clusters: Vec::new(),
        train_ids: Vec::new(),
        match_ids: Vec::new(),
    };
    let start = Instant::now();
    frames(&path, |p, line| {
        let d = models.entry(p).or_insert_with(|| {
            Drain::new(Config {
                parametrize_numeric_tokens: numeric,
                ..Config::default()
            })
            .unwrap()
        });
        let id = d.train(line).cluster_id.get();
        out.rows += 1;
        out.train_digest = digest(out.train_digest, p, id);
        if trace {
            out.train_ids.push(id);
        }
    })?;
    out.train_ns = start.elapsed().as_nanos() as u64;
    let start = Instant::now();
    frames(&path, |p, line| {
        let id = models[&p].find(line).map_or(0, |id| id.get());
        if id == 0 {
            out.unmatched += 1;
        }
        out.match_digest = digest(out.match_digest, p, id);
        if trace {
            out.match_ids.push(id);
        }
    })?;
    out.match_ns = start.elapsed().as_nanos() as u64;
    for (p, d) in &models {
        for c in d.clusters() {
            out.clusters.push(Cluster {
                partition: *p,
                id: c.id().get(),
                size: c.size(),
                template: c.template().into(),
            });
        }
    }
    out.clusters.sort_by_key(|c| (c.partition, c.id));
    serde_json::to_writer(io::stdout().lock(), &out)?;
    println!();
    Ok(())
}
