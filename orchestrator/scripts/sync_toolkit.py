#!/usr/bin/env python3
"""Preview/install/update a copied toolkit without overwriting team factories or edits."""
from __future__ import annotations

import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import shutil
import sys
import tempfile

SOURCE = Path(__file__).resolve().parents[2]
ROOT_FILES = {"README.md", "Makefile", ".gitignore"}
MANAGED_DIRS = {"orchestrator", "resources", "docs"}
EXCLUDED = {".git", "__pycache__", "node_modules", "git", "npm", ".ralph", "tasks", "auth.json", "mcp-auth.json", "trust.json", ".DS_Store", ".env"}


def allowed(path: Path) -> bool:
    return (not path.is_absolute() and ".." not in path.parts and bool(path.parts)
            and (path.parts[0] in MANAGED_DIRS or str(path) in ROOT_FILES)
            and not any(part in EXCLUDED for part in path.parts)
            and not path.name.startswith(".env.")
            and not path.name.endswith((".local.json", ".pyc", ".log")))


def digest(path: Path) -> str | None:
    if not path.exists(): return None
    if not path.is_file() or path.is_symlink(): raise ValueError(f"expected a regular file: {path}")
    return hashlib.sha256(path.read_bytes()).hexdigest()


def destination(root: Path, relative: str) -> Path:
    path = root / relative
    for parent in [path, *path.parents]:
        if parent == root: break
        if parent.is_symlink(): raise ValueError(f"refusing symlink in toolkit destination: {parent}")
    return path


def source_files(source: Path, include_factories=False):
    candidates=[]
    for directory, subdirectories, files in os.walk(source, followlinks=False):
        subdirectories[:] = sorted(name for name in subdirectories if name not in EXCLUDED)
        for name in [*subdirectories, *files]:
            path=Path(directory)/name
            if path.is_symlink() and (allowed(path.relative_to(source)) or include_factories and path.relative_to(source).parts[0]=="factories"):
                raise ValueError(f"source toolkit contains a symlink: {path.relative_to(source)}")
        candidates.extend(Path(directory)/name for name in files)
    for path in sorted(candidates):
        relative = path.relative_to(source)
        if any(part in EXCLUDED for part in relative.parts) or path.name.startswith(".env.") or path.name.endswith((".local.json", ".pyc", ".log")): continue
        if not (allowed(relative) or include_factories and relative.parts[0]=="factories"): continue
        if path.is_symlink(): raise ValueError(f"source toolkit contains a symlink: {relative}")
        if path.is_file(): yield str(relative), path


def atomic_copy(source: Path, target: Path):
    target.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix=".toolkit-writing-", dir=target.parent)
    os.close(fd)
    try:
        shutil.copy2(source, temporary)
        os.replace(temporary, target)
    finally:
        if os.path.exists(temporary): os.unlink(temporary)


def plan(source: Path, target: Path):
    manifest_path = destination(target, ".toolkit.json")
    manifest = json.loads(manifest_path.read_text()) if manifest_path.exists() else {"version":1,"files":{}}
    if not isinstance(manifest,dict) or manifest.get("version")!=1 or not isinstance(manifest.get("files"),dict):
        raise ValueError("invalid .toolkit.json")
    baseline = manifest["files"]
    for relative, value in baseline.items():
        if not allowed(Path(relative)) or not isinstance(value,str) or len(value)!=64:
            raise ValueError("invalid managed path/hash in .toolkit.json")
    incoming = dict(source_files(source))
    actions=[];conflicts=[];preserved=[];updated=dict(baseline)
    for relative in sorted(set(baseline)|set(incoming)):
        path=destination(target,relative)
        current=digest(path)
        wanted=digest(incoming[relative]) if relative in incoming else None
        old=baseline.get(relative)
        if current==wanted:
            if wanted is None: updated.pop(relative,None)
            else: updated[relative]=wanted
        elif current==old:
            actions.append({"path":relative,"action":"remove" if wanted is None else "copy"})
            if wanted is None: updated.pop(relative,None)
            else: updated[relative]=wanted
        elif wanted==old:
            preserved.append(relative)
        else:
            conflicts.append(relative)
    return {"actions":actions,"conflicts":conflicts,"preserved_local_edits":preserved,"team_factories":"preserved; compare upstream examples separately"}, {"version":1,"files":updated}


def synchronize(source: Path, target: Path, apply=False):
    if target.name!=".factory" or target==source or target.is_relative_to(source):
        raise ValueError("target must be a separate project's .factory directory")
    if target.is_symlink(): raise ValueError("target must not be a symlink")
    if not (source/"orchestrator/session.py").is_file(): raise ValueError("source is not a complete toolkit")
    if not target.exists():
        files=list(source_files(source,include_factories=True))
        report={"mode":"install","files":len(files),"conflicts":[]}
        if apply:
            if not target.parent.is_dir(): raise ValueError("project directory must already exist")
            staging=Path(tempfile.mkdtemp(prefix=".factory-install-",dir=target.parent))
            try:
                for relative,path in files: atomic_copy(path,staging/relative)
                manifest={"version":1,"files":{relative:digest(path) for relative,path in files if allowed(Path(relative))}}
                (staging/".toolkit.json").write_text(json.dumps(manifest,indent=2)+"\n")
                staging.rename(target)
            finally:
                if staging.exists(): shutil.rmtree(staging)
        return report
    if not apply:
        report,_=plan(source,target)
        return {"mode":"preview",**report}
    lock_path=destination(target,".toolkit.lock")
    with lock_path.open("a") as lock:
        fcntl.flock(lock,fcntl.LOCK_EX)
        report,manifest=plan(source,target)
        if report["conflicts"]: return {"mode":"blocked",**report}
        for action in report["actions"]:
            path=destination(target,action["path"])
            if action["action"]=="remove": path.unlink()
            else: atomic_copy(source/action["path"],path)
        fd, temporary=tempfile.mkstemp(prefix=".toolkit-manifest-",dir=target)
        try:
            with os.fdopen(fd,"w") as output: json.dump(manifest,output,indent=2);output.write("\n")
            os.replace(temporary,target/".toolkit.json")
        finally:
            if os.path.exists(temporary): os.unlink(temporary)
        return {"mode":"updated",**report}


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument("target",type=Path)
    parser.add_argument("--apply",action="store_true",help="apply the reviewed update; default only previews")
    args=parser.parse_args()
    # Resolve the parent but retain the final component for symlink rejection.
    requested=args.target.expanduser()
    target=requested.parent.resolve()/requested.name
    report=synchronize(SOURCE,target,args.apply)
    print(json.dumps(report,indent=2))
    return 1 if report.get("conflicts") else 0

if __name__=="__main__":
    try: sys.exit(main())
    except (ValueError,OSError) as error:
        print(f"toolkit update: {error}",file=sys.stderr);sys.exit(1)
