mod benchmark;

use drain3::{Change, Config, Drain, Search};
use serde::{Deserialize, Serialize};
use std::io::{self, BufRead, Write};

#[derive(Deserialize)]
struct Request {
    #[serde(default)]
    config: Config,
    lines: Vec<String>,
    queries: Vec<String>,
}
#[derive(Serialize)]
struct Step {
    id: u64,
    size: u64,
    template: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    change: Option<&'static str>,
}
#[derive(Serialize)]
struct Parameter {
    value: String,
    mask: i64,
}
#[derive(Serialize)]
struct Parsed {
    id: u64,
    params: Vec<Parameter>,
}
#[derive(Serialize)]
struct Response {
    steps: Vec<Step>,
    clusters: Vec<Step>,
    matches: Vec<[u64; 3]>,
    parsed: Vec<[Option<Parsed>; 3]>,
}
fn run(req: Request) -> Result<Response, Box<dyn std::error::Error>> {
    let mut d = Drain::new(req.config)?;
    let mut out = Response {
        steps: Vec::new(),
        clusters: Vec::new(),
        matches: Vec::new(),
        parsed: Vec::new(),
    };
    for line in &req.lines {
        let t = d.train(line);
        let c = d.cluster(t.cluster_id).unwrap();
        out.steps.push(Step {
            id: c.id().get(),
            size: c.size(),
            template: c.template().into(),
            change: Some(match t.change {
                Change::Created => "created",
                Change::TemplateChanged => "template_changed",
                Change::Unchanged => "unchanged",
            }),
        });
    }
    for c in d.clusters() {
        out.clusters.push(Step {
            id: c.id().get(),
            size: c.size(),
            template: c.template().into(),
            change: None,
        });
    }
    for q in &req.queries {
        let strategies = [Search::Never, Search::Fallback, Search::Always];
        out.matches
            .push(strategies.map(|s| d.find_with(q, s).map_or(0, |id| id.get())));
        out.parsed.push(strategies.map(|s| {
            d.parse_with(q, s).map(|p| Parsed {
                id: p.cluster_id.get(),
                params: p
                    .params
                    .into_iter()
                    .map(|v| Parameter {
                        value: v.value.into(),
                        mask: v.mask.map_or(-1, |i| i as i64),
                    })
                    .collect(),
            })
        }));
    }
    Ok(out)
}
fn main() -> Result<(), Box<dyn std::error::Error>> {
    if std::env::args().nth(1).as_deref() == Some("bench") {
        return benchmark::run();
    }
    let stdin = io::stdin();
    let mut out = io::BufWriter::new(io::stdout().lock());
    for line in stdin.lock().lines() {
        let req = serde_json::from_str(&line?)?;
        serde_json::to_writer(&mut out, &run(req)?)?;
        out.write_all(b"\n")?;
    }
    out.flush()?;
    Ok(())
}
