"""POI selection/geometry/publication tests; no network, containers or database."""
import importlib.util
import json
import math
import os
from pathlib import Path
import stat
import sys
import tempfile
import unittest
from unittest.mock import patch
import zipfile

SPEC = importlib.util.spec_from_file_location('places_osm', Path(__file__).with_name('places_osm.py'))
PLACES = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PLACES)


class TagTests(unittest.TestCase):
    def test_generic_car_services_are_not_misrepresented_as_motorcycle_workshops(self):
        self.assertIsNone(PLACES.category({'shop': 'car_repair'}))
        self.assertEqual(PLACES.category({'shop': 'car_repair', 'motorcycle': 'yes'}), 'workshop')
        self.assertEqual(PLACES.category({'shop': 'car_repair', 'service:motorcycle:repair': 'yes'}), 'workshop')
        self.assertEqual(PLACES.category({'shop': 'motorcycle', 'service:motorcycle:repair': 'yes'}), 'motorcycle_shop')
        self.assertEqual(PLACES.category({'shop': 'motorcycle_repair'}), 'workshop')

    def test_private_resident_driveway_and_inactive_features_are_excluded(self):
        for extra in ({'access': 'private'}, {'access': 'no'}, {'access': 'residents'},
                      {'motorcycle': 'no'}, {'motor_vehicle': 'no'}, {'service': 'driveway'},
                      {'access:conditional': 'yes @ (Mo-Fr)'}, {'disused': 'yes'},
                      {'abandoned:amenity': 'fuel'}):
            with self.subTest(extra=extra):
                self.assertIsNone(PLACES.category({'amenity': 'fuel', **extra}))

    def test_parking_requires_explicit_public_or_motorcycle_access(self):
        self.assertIsNone(PLACES.category({'amenity': 'parking'}))
        self.assertEqual(PLACES.category({'amenity': 'parking', 'access': 'customers'}), 'parking')
        self.assertEqual(PLACES.category({'amenity': 'parking', 'motorcycle': 'designated'}), 'parking')
        for extra in ({'parking': 'residential'}, {'parking': 'garage_boxes'},
                      {'building': 'apartments'}, {'landuse': 'residential'},
                      {'parking:condition:default': 'residents'}, {'access': 'permit'},
                      {'access': 'private', 'motorcycle': 'yes'}):
            self.assertIsNone(PLACES.category({'amenity': 'parking', 'access': 'yes', **extra}))

    def test_nonbusiness_names_are_not_invented_and_source_fields_remain_unverified(self):
        self.assertIsNone(PLACES.fields({'amenity': 'fuel'}))
        record = PLACES.fields({'amenity': 'fuel', 'brand': 'Source brand', 'addr:city': 'Алматы',
            'addr:street': 'Абай', 'addr:housenumber': '150', 'contact:website': 'https://example.kz',
            'contact:phone': '+7 700 000 0000', 'opening_hours': '24/7', 'description': 'Source description'})
        self.assertEqual(record['name'], 'Source brand')
        self.assertEqual(record['address'], 'Абай 150')
        self.assertEqual(record['description'], 'Source description')
        self.assertEqual(record['opening_hours'], '24/7')
        self.assertEqual(record['website'], 'https://example.kz')
        self.assertNotIn('verified', record)
        self.assertNotIn('available', record)

    def test_unsafe_urls_and_multibyte_field_overflow_do_not_reach_import(self):
        for website in ('javascript:alert(1)', 'https://user:password@example.kz', 'https://example.kz/a b', 'http://[invalid', 'https://example.kz/%zz'):
            record = PLACES.fields({'shop': 'tyres', 'name': 'Ә' * 200, 'website': website})
            self.assertEqual(record['website'], '')
            self.assertLessEqual(len(record['name'].encode()), 200)
            self.assertTrue(record['name'])
        self.assertEqual(PLACES.category({'amenity': 'meeting_point'}), 'meeting_spot')
        self.assertIsNone(PLACES.category({'tourism': 'picnic_site'}))
        record = PLACES.fields({'shop': 'motorcycle', 'name': 'Source business',
            'website': 'https://example.kz/%D0%90%D0%B1%D0%B0%D0%B9?lang=kk'})
        self.assertEqual(record['website'], 'https://example.kz/%D0%90%D0%B1%D0%B0%D0%B9?lang=kk')


class GeometryAndDateTests(unittest.TestCase):
    def test_closed_way_uses_area_centroid_and_open_way_uses_length_midpoint(self):
        square = [(76, 43), (76.02, 43), (76.02, 43.02), (76, 43.02), (76, 43)]
        point = PLACES.representative(square, True)
        self.assertAlmostEqual(point[0], 76.01)
        self.assertAlmostEqual(point[1], 43.01)
        # An extra vertex close to the start must not bias a line's position.
        point = PLACES.representative([(76, 43), (76.001, 43), (76.02, 43)], False)
        self.assertAlmostEqual(point[0], 76.01)
        self.assertAlmostEqual(point[1], 43)

    def test_concave_polygon_falls_back_to_its_boundary(self):
        ring = [(76, 43), (76.03, 43), (76.03, 43.03), (76.02, 43.03),
                (76.02, 43.01), (76.01, 43.01), (76.01, 43.03), (76, 43.03), (76, 43)]
        point = PLACES.representative(ring, True)
        self.assertTrue(any(abs((point[0] - a[0]) * (b[1] - a[1]) - (point[1] - a[1]) * (b[0] - a[0])) < 1e-12
                            and min(a[0], b[0]) <= point[0] <= max(a[0], b[0])
                            and min(a[1], b[1]) <= point[1] <= max(a[1], b[1])
                            for a, b in zip(ring, ring[1:])))

    def test_invalid_incomplete_or_country_spanning_geometry_is_skipped(self):
        for points in ([(76, 43)], [(math.nan, 43), (76, 43)], [(76, 43), (77, 44)], [(30, 43), (30.01, 43)]):
            self.assertIsNone(PLACES.representative(points, False))

    def test_source_date_comes_from_snapshot_not_current_clock(self):
        class Header(dict):
            has_multiple_object_versions = False
        header = Header(osmosis_replication_timestamp='2026-11-03T21:24:23Z')
        self.assertEqual(PLACES.snapshot_date(header, '2026-11-03'), '2026-11-03')
        with self.assertRaisesRegex(ValueError, 'differs'):
            PLACES.snapshot_date(header, '2026-11-09')
        with self.assertRaisesRegex(ValueError, 'timestamp'):
            PLACES.snapshot_date(Header())
        header.has_multiple_object_versions = True
        with self.assertRaisesRegex(ValueError, 'History'):
            PLACES.snapshot_date(header)


class OutputAndWheelTests(unittest.TestCase):
    def test_source_modified_during_hash_is_rejected_before_parser_load(self):
        with tempfile.TemporaryDirectory() as tmp:
            source = Path(tmp) / 'source.osm.pbf'; source.write_bytes(b'original')
            expected = PLACES.sha256(source)
            def changed(_path):
                source.write_bytes(b'different source bytes')
                return expected
            with patch.object(PLACES, 'sha256', side_effect=changed), patch.object(PLACES, 'load_osmium') as parser:
                with self.assertRaisesRegex(ValueError, 'changed while'):
                    PLACES.extract(source, Path(tmp) / 'places.ndjson', expected, '2026-11-03')
            parser.assert_not_called()
            self.assertFalse((Path(tmp) / 'places.ndjson').exists())

    def test_output_contract_is_stable_public_readable_and_duplicate_safe(self):
        with tempfile.TemporaryDirectory() as tmp:
            output = Path(tmp) / 'places.ndjson'
            writer = PLACES.NDJSON(output)
            try:
                data = PLACES.fields({'amenity': 'fuel', 'name': 'OSM fixture'})
                writer.add('node', 42, data, (76.95, 43.24), '2026-11-03')
                with self.assertRaisesRegex(ValueError, 'Duplicate'):
                    writer.add('node', 42, data, (76.95, 43.24), '2026-11-03')
                writer.publish()
                self.assertEqual(output.stat().st_mode & 0o777, 0o644)
                record = json.loads(output.read_text())
                self.assertEqual(set(record), {'source_ref', 'source_date', 'name', 'category', 'lat', 'lon',
                    'city', 'address', 'description', 'website', 'phone', 'opening_hours'})
                self.assertEqual(record['source_ref'], 'node/42')
                self.assertEqual(PLACES.sha256(output), writer.hash.hexdigest())
            finally:
                writer.close()

    def test_exceeded_output_budget_never_publishes_partial_file_or_overwrites_existing(self):
        with tempfile.TemporaryDirectory() as tmp:
            output = Path(tmp) / 'places.ndjson'
            writer = PLACES.NDJSON(output)
            try:
                with patch.object(PLACES, 'MAX_BYTES', 1):
                    with self.assertRaisesRegex(ValueError, 'no partial'):
                        writer.add('way', 1, PLACES.fields({'shop': 'tyres', 'name': 'Fixture'}), (76, 43), '2026-11-03')
                self.assertFalse(output.exists())
                output.write_text('existing data')
                with self.assertRaises(ValueError):
                    PLACES.NDJSON(output)
                self.assertEqual(output.read_text(), 'existing data')
            finally:
                writer.close()

    def wheel(self, folder, path='package/data.py', symlink=False):
        archive = folder / 'fixture.whl'
        with zipfile.ZipFile(archive, 'w') as output:
            member = zipfile.ZipInfo(path)
            member.external_attr = ((stat.S_IFLNK if symlink else stat.S_IFREG) | 0o644) << 16
            output.writestr(member, b'fixture')
        return {'filename': archive.name, 'bytes': archive.stat().st_size, 'sha256': PLACES.sha256(archive)}

    def test_pinned_wheel_extraction_rejects_traversal_links_and_hash_changes(self):
        with tempfile.TemporaryDirectory() as tmp:
            folder = Path(tmp)
            for index, (name, link) in enumerate((('../escape', False), ('package/link', True))):
                manifest = self.wheel(folder, name, link)
                with patch.object(PLACES, 'require_runtime'), patch.object(PLACES, 'wheel_manifest', return_value=[manifest]):
                    with self.assertRaisesRegex(ValueError, 'unsafe'):
                        PLACES.prepare_site(folder, folder / f'site{index}')
                    self.assertFalse((folder / f'site{index}').exists())
            manifest = self.wheel(folder)
            manifest['sha256'] = '0' * 64
            with patch.object(PLACES, 'require_runtime'), patch.object(PLACES, 'wheel_manifest', return_value=[manifest]):
                with self.assertRaisesRegex(ValueError, 'pinned'):
                    PLACES.prepare_site(folder, folder / 'site')

    def test_valid_pinned_wheel_publishes_private_runtime_without_overwriting_it(self):
        with tempfile.TemporaryDirectory() as tmp:
            folder = Path(tmp)
            manifest = self.wheel(folder)
            with patch.object(PLACES, 'require_runtime'), patch.object(PLACES, 'wheel_manifest', return_value=[manifest]):
                result = PLACES.prepare_site(folder, folder / 'site')
                self.assertEqual(result['files'], 1)
                self.assertEqual((folder / 'site/package/data.py').read_bytes(), b'fixture')
                with self.assertRaisesRegex(ValueError, 'replace'):
                    PLACES.prepare_site(folder, folder / 'site')


@unittest.skipUnless(importlib.util.find_spec('osmium') is not None and sys.platform == 'linux' and sys.version_info[:2] == (3, 12),
                     'Run with the staged pinned parser inside the capped Linux/amd64 job')
class ParserFixtureTests(unittest.TestCase):
    def test_real_pbf_two_pass_fixture_keeps_public_nodes_and_complete_ways_only(self):
        osmium = PLACES.load_osmium()
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'fixture.osm.pbf'
            header = osmium.io.Header(); header.set('osmosis_replication_timestamp', '2026-11-03T21:24:23Z')
            with osmium.SimpleWriter(str(path), header=header) as writer:
                for node_id, point in [(1, (76, 43)), (2, (76.02, 43)), (3, (76.02, 43.02)), (4, (76, 43.02))]:
                    writer.add_node(osmium.osm.mutable.Node(id=node_id, location=point))
                writer.add_node(osmium.osm.mutable.Node(id=10, location=(76.01, 43.01), tags={'amenity': 'fuel', 'name': 'Public fixture'}))
                writer.add_node(osmium.osm.mutable.Node(id=11, location=(76.01, 43.01), tags={'amenity': 'parking', 'access': 'private', 'name': 'Private fixture'}))
                writer.add_way(osmium.osm.mutable.Way(id=20, nodes=[1, 2, 3, 4, 1], tags={'shop': 'motorcycle', 'name': 'Way fixture'}))
                writer.add_way(osmium.osm.mutable.Way(id=21, nodes=[1, 9999], tags={'shop': 'tyres', 'name': 'Missing node fixture'}))
            output = Path(tmp) / 'places.ndjson'
            result = PLACES.extract(path, output, PLACES.sha256(path), '2026-11-03')
            rows = [json.loads(line) for line in output.read_text().splitlines()]
            self.assertEqual([row['source_ref'] for row in rows], ['node/10', 'way/20'])
            self.assertEqual(result['skipped']['ways_missing_nodes'], 1)
            self.assertTrue(all(row['source_date'] == '2026-11-03' for row in rows))


if __name__ == '__main__':
    unittest.main()
