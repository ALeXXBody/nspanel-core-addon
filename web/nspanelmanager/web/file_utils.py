import hashlib
import os
import re

from django.core.files.storage import FileSystemStorage

# MD5 checksums are requested by every panel at boot and before every update.
# Each checksum used to re-read and hash the whole multi-MB file. Cache by
# (mtime, size) so the same file keeps its cached checksum until it changes.
_md5_cache = {}

# Header for Content-Length etc where a real file object can't estimate size.
_MD5_CHUNK_SIZE = 64 * 1024

_RANGE_PATTERN = re.compile(r"^bytes=\s*(\d*)\s*-(\d*)\s*$")


# Range header parser lives next to its only consumer (_serve_file_range).
_RANGE_PATTERN = re.compile(r"^bytes=\s*(\d*)\s*-(\d*)\s*$")


def compute_file_md5sum(path):
    """Streamed md5 of a file object (no single .read() of the whole file)."""
    digest = hashlib.md5()
    for chunk in iter(lambda: path.read(_MD5_CHUNK_SIZE), b""):
        digest.update(chunk)
    return digest.hexdigest()


def get_cached_file_md5sum(path_on_storage):
    """
    MD5 checksum for a FileSystemStorage path, cached by (mtime, size).
    Returns None if the file does not exist.
    """
    fs = FileSystemStorage()
    if not fs.exists(path_on_storage):
        return None
    full_path = fs.path(path_on_storage)
    stat = os.stat(full_path)
    key = (full_path, stat.st_mtime_ns, stat.st_size)
    cached = _md5_cache.get(full_path)
    if cached is not None and cached[0] == key[1] and cached[1] == key[2]:
        return cached[2]
    with open(full_path, "rb") as f:
        md5sum = compute_file_md5sum(f)
    _md5_cache[full_path] = (stat.st_mtime_ns, stat.st_size, md5sum)
    return md5sum
