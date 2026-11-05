"""Include the import-guard safety suite in normal deployment test discovery."""
import importlib.util
from pathlib import Path


def load_tests(loader, _tests, _pattern):
    source = Path(__file__).with_name('osm-benchmark') / 'test_run.py'
    spec = importlib.util.spec_from_file_location('osm_guard_safety_tests', source)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return loader.loadTestsFromModule(module)
