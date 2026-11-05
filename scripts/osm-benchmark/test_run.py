"""Safety-contract tests; all Docker, process and host operations are mocked."""

from copy import deepcopy
import importlib.util
import json
from pathlib import Path
import signal
import subprocess
import tempfile
import unittest
from unittest.mock import Mock, call, patch


MODULE_SPEC = importlib.util.spec_from_file_location(
    "osm_benchmark_supervisor", Path(__file__).with_name("run.py")
)
supervisor = importlib.util.module_from_spec(MODULE_SPEC)
MODULE_SPEC.loader.exec_module(supervisor)
GIB = 1024 ** 3
MIB = 1024 ** 2


def healthy_state():
    sample = {"time": 1000, "available": 10 * GIB, "disk_free": 40 * GIB,
              "swap_used": 64 * MIB}
    readiness = {"okay": True, "seconds": 0.02}
    current = {
        "existing-app": {"id": "app-id", "started": "start-a", "restarts": 0,
                         "running": True, "health": "healthy", "benchmark": None},
        "existing-worker": {"id": "worker-id", "started": "start-b", "restarts": 1,
                            "running": True, "health": None, "benchmark": None},
    }
    baseline = {"host": deepcopy(sample), "readiness_p95": 0.02,
                "containers": deepcopy(current)}
    return sample, readiness, baseline, current


class GuardReasonsTests(unittest.TestCase):
    def setUp(self):
        self.sample, self.readiness, self.baseline, self.current = healthy_state()

    def reasons(self):
        return supervisor.guard_reasons(
            self.sample, self.readiness, self.baseline, self.current
        )

    def test_healthy_baseline_and_new_containers_do_not_abort(self):
        self.current["benchmark-import"] = {
            "id": "bench-id", "started": "new", "restarts": 0,
            "running": True, "health": None, "benchmark": "this-run",
        }
        self.assertEqual([], self.reasons())

    def test_ram_floor_includes_exactly_three_gib(self):
        self.sample["available"] = 3 * GIB
        self.assertEqual([], self.reasons())
        self.sample["available"] -= 1
        self.assertEqual(["available RAM below 3 GiB"], self.reasons())

    def test_disk_floor_includes_exactly_twenty_gib(self):
        self.sample["disk_free"] = 20 * GIB
        self.assertEqual([], self.reasons())
        self.sample["disk_free"] -= 1
        self.assertEqual(["root filesystem free space below 20 GiB"], self.reasons())

    def test_swap_guard_measures_growth_from_baseline(self):
        self.sample["swap_used"] = self.baseline["host"]["swap_used"] + 256 * MIB
        self.assertEqual([], self.reasons())
        self.sample["swap_used"] += 1
        self.assertEqual(["host swap use increased by over 256 MiB"], self.reasons())

    def test_failed_readiness_aborts_even_if_fast(self):
        self.readiness.update(okay=False, seconds=0.001)
        self.assertEqual(
            ["application readiness failed or exceeded latency guard"], self.reasons()
        )

    def test_readiness_latency_uses_one_second_floor(self):
        self.readiness["seconds"] = 1.0
        self.assertEqual([], self.reasons())
        self.readiness["seconds"] = 1.001
        self.assertEqual(
            ["application readiness failed or exceeded latency guard"], self.reasons()
        )

    def test_readiness_latency_respects_five_times_baseline(self):
        self.baseline["readiness_p95"] = 0.4
        self.readiness["seconds"] = 2.0
        self.assertEqual([], self.reasons())
        self.readiness["seconds"] = 2.001
        self.assertEqual(
            ["application readiness failed or exceeded latency guard"], self.reasons()
        )

    def test_explicit_readiness_limit_overrides_legacy_latency_allowance(self):
        self.baseline.update(readiness_limit=0.25, readiness_p95=0.4)
        self.readiness["seconds"] = 0.25
        self.assertEqual([], self.reasons())
        self.readiness["seconds"] = 0.250001
        self.assertEqual(
            ["application readiness failed or exceeded latency guard"], self.reasons()
        )

    def test_existing_container_identity_start_and_restarts_are_guarded(self):
        for field, changed in (("id", "replacement"), ("started", "restarted"),
                               ("restarts", 2)):
            with self.subTest(field=field):
                current = deepcopy(self.current)
                current["existing-app"][field] = changed
                self.assertIn(
                    "existing container identity/restart changed: existing-app",
                    supervisor.guard_reasons(self.sample, self.readiness, self.baseline, current),
                )

    def test_disappeared_container_is_guarded(self):
        # containers() inventories running containers; a stopped one disappears.
        del self.current["existing-worker"]
        self.assertIn(
            "existing container identity/restart changed: existing-worker", self.reasons()
        )

    def test_healthy_container_must_remain_healthy(self):
        for health in ("unhealthy", "starting", None):
            with self.subTest(health=health):
                self.current["existing-app"]["health"] = health
                self.assertIn("existing container health regressed: existing-app", self.reasons())


class BaselineTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="bikers-osm-kz-tests-")
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.sample, self.readiness, _, self.current = healthy_state()

    def test_prepare_records_healthy_host_and_existing_containers(self):
        with patch.object(supervisor, "probe", return_value=self.readiness) as probe, \
                patch.object(supervisor, "host", return_value=self.sample), \
                patch.object(supervisor, "containers", return_value=self.current), \
                patch.object(supervisor.time, "sleep"), patch("builtins.print"):
            supervisor.prepare(self.root)
        saved = json.loads((self.root / "baseline.json").read_text())
        self.assertEqual(12, probe.call_count)
        self.assertEqual(self.current, saved["containers"])
        self.assertEqual(self.sample, saved["host"])
        self.assertEqual([0.02] * 12, saved["readiness_samples"])

    def test_prepare_refuses_an_unready_application_before_accepting_baseline(self):
        with patch.object(supervisor, "probe", return_value={"okay": False}), \
                patch.object(supervisor, "host") as host:
            with self.assertRaisesRegex(RuntimeError, "not ready at baseline"):
                supervisor.prepare(self.root)
        self.assertFalse((self.root / "baseline.json").exists())
        host.assert_not_called()

    def test_prepare_requires_more_headroom_than_running_guard(self):
        for field, value in (("available", 8 * GIB - 1), ("disk_free", 30 * GIB - 1)):
            with self.subTest(field=field):
                sample = {**self.sample, field: value}
                root = self.root / field
                with patch.object(supervisor, "probe", return_value=self.readiness), \
                        patch.object(supervisor, "host", return_value=sample), \
                        patch.object(supervisor.time, "sleep"):
                    with self.assertRaisesRegex(RuntimeError, "Insufficient initial headroom"):
                        supervisor.prepare(root)
                self.assertFalse((root / "baseline.json").exists())


class ControlFileTests(unittest.TestCase):
    def test_save_follows_broken_control_symlink_and_preserves_link_on_updates(self):
        with tempfile.TemporaryDirectory(prefix="bikers-osm-kz-tests-") as temporary:
            root = Path(temporary)
            controls = root / "control-files"
            controls.mkdir()
            target = controls / "watch.json"
            link = root / "watch.json"
            link.symlink_to(target)
            self.assertTrue(link.is_symlink())
            self.assertFalse(link.exists(), "initial control target intentionally does not exist")

            supervisor.save(link, {"time": 1000, "reason": "healthy"})
            self.assertTrue(link.is_symlink())
            self.assertEqual(target, link.readlink())
            self.assertEqual({"time": 1000, "reason": "healthy"}, json.loads(target.read_text()))

            supervisor.save(link, {"time": 1002, "reason": "still healthy"})
            self.assertTrue(link.is_symlink())
            self.assertEqual({"time": 1002, "reason": "still healthy"}, json.loads(link.read_text()))
            self.assertFalse((root / "watch.new").exists())
            self.assertFalse((controls / "watch.new").exists())


class StopOwnedTests(unittest.TestCase):
    def setUp(self):
        self.root = Path("/tmp/bikers-osm-kz-test-owner")

    def fake_docker(self, fail_to_stop=False):
        state = {
            "existing-app": {"owned": False, "running": True},
            "own-import": {"owned": True, "running": True},
            "own-prior": {"owned": True, "running": False},
        }

        def run(args, **kwargs):
            if args[:2] == ["docker", "ps"]:
                self.assertEqual(
                    ["--filter", f"label={supervisor.LABEL}={self.root.name}"], args[-2:]
                )
                candidates = [ident for ident, data in state.items() if data["owned"]]
                if args[2] == "-q":
                    candidates = [ident for ident in candidates if state[ident]["running"]]
                else:
                    self.assertEqual("-aq", args[2])
                return subprocess.CompletedProcess(args, 0, "\n".join(candidates), "")
            self.assertEqual(["docker", "stop", "-t", "8"], args[:4])
            self.assertEqual(60, kwargs["timeout"])
            self.assertTrue(args[4:])
            for ident in args[4:]:
                self.assertTrue(state[ident]["owned"], "unlabeled application must never be stopped")
                if not fail_to_stop:
                    state[ident]["running"] = False
            return subprocess.CompletedProcess(args, 0, "", "")

        return state, Mock(side_effect=run)

    def test_stops_only_owned_containers_and_verifies_none_remain_running(self):
        state, docker = self.fake_docker()
        with patch.object(supervisor, "command", docker):
            supervisor.stop_owned(self.root)
        self.assertTrue(state["existing-app"]["running"])
        self.assertFalse(state["own-import"]["running"])
        self.assertFalse(state["own-prior"]["running"])
        self.assertEqual(["docker", "ps", "-q", "--filter",
                          f"label={supervisor.LABEL}={self.root.name}"], docker.call_args.args[0])

    def test_running_owned_container_after_stop_is_an_error(self):
        state, docker = self.fake_docker(fail_to_stop=True)
        with patch.object(supervisor, "command", docker):
            with self.assertRaisesRegex(RuntimeError, "did not stop"):
                supervisor.stop_owned(self.root)
        self.assertTrue(state["existing-app"]["running"])

    def test_no_owned_containers_still_checks_running_filter(self):
        docker = Mock(return_value=subprocess.CompletedProcess([], 0, "", ""))
        with patch.object(supervisor, "command", docker):
            supervisor.stop_owned(self.root)
        self.assertEqual(2, docker.call_count)
        self.assertTrue(all(call.args[0][:2] == ["docker", "ps"] for call in docker.call_args_list))

    def test_docker_stop_failure_is_propagated(self):
        docker = Mock(side_effect=[
            subprocess.CompletedProcess([], 0, "own-import\n", ""),
            subprocess.CompletedProcess([], 1, "", "daemon unavailable"),
        ])
        with patch.object(supervisor, "command", docker):
            with self.assertRaises(subprocess.CalledProcessError):
                supervisor.stop_owned(self.root)


class ContainerSnapshotTests(unittest.TestCase):
    def test_failed_listing_does_not_become_an_empty_existing_app_baseline(self):
        docker = Mock(return_value=subprocess.CompletedProcess([], 1, '', 'daemon unavailable'))
        with patch.object(supervisor, 'command', docker):
            with self.assertRaises(subprocess.CalledProcessError):
                supervisor.containers()
        self.assertEqual(docker.call_count, 1)

    def test_successful_empty_listing_is_distinct_from_a_failed_listing(self):
        docker = Mock(return_value=subprocess.CompletedProcess([], 0, '', ''))
        with patch.object(supervisor, 'command', docker):
            self.assertEqual(supervisor.containers(), {})
        self.assertEqual(docker.call_count, 1)


class Clock:
    def __init__(self):
        self.elapsed = 0

    def time(self):
        return 1000 + self.elapsed

    def monotonic(self):
        return self.elapsed


class PendingTelemetryPool:
    """A deterministic hung Docker sample; no background threads or Docker."""
    def __init__(self, max_workers):
        self.max_workers = max_workers
        self.future = Mock()
        self.future.done.return_value = False
        self.submissions = []

    def submit(self, function):
        self.submissions.append(function)
        return self.future


class WatchTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="bikers-osm-kz-tests-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.sample, self.readiness, self.baseline, _ = healthy_state()
        (self.root / "baseline.json").write_text(json.dumps(self.baseline))
        self.clock = Clock()
        self.pool = PendingTelemetryPool(max_workers=1)

    def sample_now(self, root):
        return {**self.sample, "time": self.clock.time()}

    def patches(self, sleep, readiness=None):
        return (
            patch.object(supervisor, "ThreadPoolExecutor", return_value=self.pool),
            patch.object(supervisor.time, "time", side_effect=self.clock.time),
            patch.object(supervisor.time, "monotonic", side_effect=self.clock.monotonic),
            patch.object(supervisor.time, "sleep", side_effect=sleep),
            patch.object(supervisor, "host", side_effect=self.sample_now),
            patch.object(supervisor, "probe", side_effect=readiness or (lambda: self.readiness)),
        )

    def run_with(self, patches):
        for item in patches:
            item.start()
            self.addCleanup(item.stop)

    def test_pending_docker_telemetry_does_not_stale_host_heartbeat(self):
        def sleep(seconds):
            self.clock.elapsed += seconds
            if self.clock.elapsed >= 24:
                (self.root / "finished.json").write_text("{}")

        self.run_with(self.patches(sleep))
        with patch.object(supervisor, "stop_owned") as stop:
            supervisor.watch(self.root, duration=100)
        samples = [json.loads(line) for line in (self.root / "samples.jsonl").read_text().splitlines()]
        self.assertGreaterEqual(len(samples), 12)
        self.assertTrue(all(not sample["reasons"] for sample in samples))
        self.assertTrue(all(later["time"] - earlier["time"] == 2
                            for earlier, later in zip(samples, samples[1:])))
        self.assertEqual(1, len(self.pool.submissions), "never overlap stalled Docker jobs")
        self.assertFalse((self.root / "abort.json").exists())
        supervisor.ensure_watch(self.root)
        stop.assert_not_called()

    def test_docker_telemetry_stall_eventually_aborts_and_stops_owned(self):
        def sleep(seconds):
            self.clock.elapsed += seconds

        self.run_with(self.patches(sleep))
        with patch.object(supervisor, "stop_owned") as stop:
            with self.assertRaisesRegex(RuntimeError, "Docker telemetry unavailable"):
                supervisor.watch(self.root, duration=1000)
        self.assertGreater(self.clock.elapsed, 60)
        self.assertLess(self.clock.elapsed, 75)
        stop.assert_called_once_with(self.root)
        self.assertIn("Docker telemetry unavailable", json.loads((self.root / "abort.json").read_text())["reason"])

    def test_readiness_strikes_reset_after_a_healthy_sample(self):
        readings = iter([False, False, True, False, False, False])

        def readiness():
            return {"okay": next(readings), "seconds": 0.01}

        def sleep(seconds):
            self.clock.elapsed += seconds

        self.run_with(self.patches(sleep, readiness))
        with patch.object(supervisor, "stop_owned") as stop:
            with self.assertRaisesRegex(RuntimeError, "application readiness failed"):
                supervisor.watch(self.root, duration=100)
        self.assertEqual(6, len((self.root / "samples.jsonl").read_text().splitlines()))
        stop.assert_called_once_with(self.root)

    def assert_readiness_stops_immediately(self, readiness):
        self.baseline.update(readiness_limit=0.25, fail_fast_readiness=True)
        (self.root / "baseline.json").write_text(json.dumps(self.baseline))

        def sleep(seconds):
            self.clock.elapsed += seconds

        self.run_with(self.patches(sleep, lambda: readiness))
        with patch.object(supervisor, "stop_owned") as stop:
            with self.assertRaisesRegex(RuntimeError, "application readiness failed"):
                supervisor.watch(self.root, duration=100)
        self.assertEqual(0, self.clock.elapsed, "fail-fast mode must not wait for more samples")
        self.assertEqual(1, len((self.root / "samples.jsonl").read_text().splitlines()))
        stop.assert_called_once_with(self.root)

    def test_fail_fast_readiness_stops_on_first_failed_response(self):
        self.assert_readiness_stops_immediately({"okay": False, "seconds": 0.001})

    def test_fail_fast_readiness_stops_on_first_response_above_strict_limit(self):
        self.assert_readiness_stops_immediately({"okay": True, "seconds": 0.251})

    def test_completed_telemetry_health_regression_aborts_without_waiting_three_strikes(self):
        current = deepcopy(self.baseline["containers"])
        current["existing-app"]["health"] = "unhealthy"
        self.pool.future.done.return_value = True
        self.pool.future.result.return_value = {
            "time": 1002, "containers": current, "cgroups": {}, "bytes": 0,
        }

        def sleep(seconds):
            self.clock.elapsed += seconds

        self.run_with(self.patches(sleep))
        with patch.object(supervisor, "stop_owned") as stop:
            with self.assertRaisesRegex(RuntimeError, "existing container health regressed"):
                supervisor.watch(self.root, duration=100)
        self.assertEqual(2, len((self.root / "samples.jsonl").read_text().splitlines()))
        stop.assert_called_once_with(self.root)


class CleanupTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="bikers-osm-kz-tests-")
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        (self.root / "logs").mkdir()

    def test_lease_always_stops_owned_after_a_watchdog_failure(self):
        with patch.object(supervisor, "ensure_watch", side_effect=RuntimeError("stale lease")), \
                patch.object(supervisor, "stop_owned") as stop:
            with self.assertRaisesRegex(RuntimeError, "stale lease"):
                supervisor.lease(self.root, duration=100)
        stop.assert_called_once_with(self.root)
        self.assertIn("stale lease", json.loads((self.root / "abort.json").read_text())["reason"])

    def test_finished_lease_also_verifies_owned_containers_stopped(self):
        (self.root / "finished.json").write_text("{}")
        with patch.object(supervisor, "stop_owned") as stop:
            supervisor.lease(self.root, duration=100)
        stop.assert_called_once_with(self.root)

    def test_lease_preserves_first_abort_reason_when_it_observes_existing_abort(self):
        controls = self.root / "control-files"
        controls.mkdir()
        target = controls / "abort.json"
        link = self.root / "abort.json"
        link.symlink_to(target)
        first_reason = {"time": 777, "reason": "application readiness exceeded 250 ms"}
        supervisor.save(link, first_reason)
        original_bytes = target.read_bytes()
        with patch.object(supervisor, "save", wraps=supervisor.save) as save, \
                patch.object(supervisor, "stop_owned") as stop:
            with self.assertRaisesRegex(RuntimeError, "Benchmark aborted"):
                supervisor.lease(self.root, duration=100)
        save.assert_not_called()
        stop.assert_called_once_with(self.root)
        self.assertTrue(link.is_symlink())
        self.assertEqual(original_bytes, target.read_bytes())
        self.assertEqual(first_reason, json.loads(link.read_text()))

    def test_watch_heartbeat_allows_fifteen_seconds_but_rejects_older(self):
        (self.root / "watch.json").write_text(json.dumps({"time": 1000}))
        with patch.object(supervisor.time, "time", return_value=1015):
            supervisor.ensure_watch(self.root)
        with patch.object(supervisor.time, "time", return_value=1015.001):
            with self.assertRaisesRegex(RuntimeError, "heartbeat is stale"):
                supervisor.ensure_watch(self.root)

    def test_existing_abort_receipt_prevents_continuation(self):
        (self.root / "abort.json").write_text(json.dumps({"reason": "prior guard failure"}))
        with self.assertRaisesRegex(RuntimeError, "Benchmark aborted"):
            supervisor.ensure_watch(self.root)

    def test_host_child_is_terminated_even_when_docker_cleanup_fails(self):
        process = Mock(pid=98765)
        process.poll.return_value = None
        with patch.object(supervisor, "ensure_watch", side_effect=[None, RuntimeError("stale")]), \
                patch.object(supervisor.subprocess, "Popen", return_value=process), \
                patch.object(supervisor, "stop_owned", side_effect=RuntimeError("stop failed")), \
                patch.object(supervisor.os, "killpg") as kill:
            with self.assertRaisesRegex(RuntimeError, "stop failed"):
                supervisor.execute(self.root, "test-command", ["never-executed"], seconds=10)
        kill.assert_called_once_with(process.pid, signal.SIGTERM)
        process.wait.assert_called_once_with(timeout=10)

    def test_unresponsive_host_child_gets_sigkill(self):
        process = Mock(pid=98765)
        process.poll.return_value = None
        process.wait.side_effect = subprocess.TimeoutExpired("never-executed", 10)
        with patch.object(supervisor, "ensure_watch", side_effect=[None, RuntimeError("stale")]), \
                patch.object(supervisor.subprocess, "Popen", return_value=process), \
                patch.object(supervisor, "stop_owned") as stop, \
                patch.object(supervisor.os, "killpg") as kill:
            with self.assertRaisesRegex(RuntimeError, "stale"):
                supervisor.execute(self.root, "test-command", ["never-executed"], seconds=10)
        self.assertEqual([call(process.pid, signal.SIGTERM), call(process.pid, signal.SIGKILL)], kill.call_args_list)
        stop.assert_called_once_with(self.root)

    def test_execution_receipt_does_not_clobber_same_named_measurement_output(self):
        results = self.root / "results"
        results.mkdir()
        output = results / "routing-query.json"
        measurements = {"requests": [{"elapsed_ms": 42}], "successful": 1}
        output.write_text(json.dumps(measurements))
        original_bytes = output.read_bytes()
        process = Mock(pid=98765, returncode=0)
        process.poll.return_value = 0
        with patch.object(supervisor, "ensure_watch"), \
                patch.object(supervisor.subprocess, "Popen", return_value=process), \
                patch("builtins.print"):
            supervisor.execute(self.root, "routing-query", ["never-executed"], seconds=10)
        self.assertEqual(original_bytes, output.read_bytes())
        receipt = json.loads((results / "routing-query-receipt.json").read_text())
        self.assertEqual("routing-query", receipt["name"])
        self.assertEqual(0, receipt["exit_code"])


if __name__ == "__main__":
    unittest.main()
