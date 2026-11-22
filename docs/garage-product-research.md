# BIKERS.KZ garage and Astana pilot research

Checked 22 November 2026. The review uses the live public BIKERS.KZ feeds, the
app source, and public product pages, guides, and screenshots from comparable
services.

## What riders see today

The public feed returned five upcoming rides, including one in Astana on
26 November. All five were eligible for fallback artwork. The existing city
photo fallback gives a ride a sense of place. A real cover or an album from a
recent local ride can show what riders actually do together. Keep upcoming
Astana rides easy to spot and use real local ride photos and route stories to
give the feed something useful between events. [Live rides feed](https://bikers.kz/api/v1/rides) · [City photo fallback](design-review.md#city-photo-fallback--20-november-2026)

The public Motorcycles feed returned 13 listings. Its associated public
galleries returned no owner photos or selected covers. A card can still fall
back to a verified model reference when its year matches. Before this change,
the signed-in app linked to My Garage under Me. The add and edit form collects
make, model, and year. An owner can look up a model or enter it manually,
upload photos, opt in to a public page, and keep a private maintenance log.
[Live motorcycle feed](https://bikers.kz/api/v1/bikes) · [Garage screens](../apps/web/src/pages/Garage.tsx) · [Bike media](../apps/web/src/components/BikePhotos.tsx)

## Comparable products

| Product | Published pattern | Application to BIKERS.KZ |
| --- | --- | --- |
| Cardo Ride | A Garage icon on the Go screen opens bikes directly. Riders can assign a motorcycle to a saved trip and see trips and distance by bike. [Product guide](https://journal.riserapp.com/how-to-use-the-riser-app/) | Give My Garage a direct navigation entry. Consider optional motorcycle assignment to saved rides after the garage is easy to use. |
| MotoClub and Moto Garage | MotoClub shows «Гараж» as a main section and supports manually entering rare and custom bikes. Moto Garage shows photo cards with status and odometer, a year/make/model lookup, and dedicated service and modifications screens. [MotoClub](https://motoclub.app/site/) · [Moto Garage](https://motogarage.io/) | Use bike covers, readable identity, and quick add. Keep rare bikes easy to save and give maintenance and mods their own space. |
| REVGARAGE | Owners select the photos and details for a shared build. VIN, plate, documents, service records, and costs stay in the garage. [Community guide](https://revgarage.app/vehicle-community) | Continue opt-in publishing and make it clear which fields a public bike page shows. |
| Yamaha and Kolesa.kz | Yamaha groups verified specs by engine, chassis, and dimensions and distinguishes factory variants. Astana listings on Kolesa.kz show how local riders scan year, displacement, category, and fuel across a wider range of makes. [Yamaha brochure](https://www.yamaha-motor.eu/content/dam/yme/language-masters/en/eicma-2025/digital-brochures/2026_EICMA_MC_BROCHURE_EN.pdf) · [Kolesa.kz Astana](https://kolesa.kz/mototehnika/motocikly/astana/) | Show a few useful facts at a glance. Use verified year, trim, and market for any catalog specs. Include older imports and locally listed Chinese makes when evaluating model coverage. |
| MotoMoto and BikePost | Rider photos and stories give readers a reason to browse between events. BikePost shows recent posts alongside its upcoming-events panel. [MotoMoto](https://motomoto.su/) · [BikePost](https://bikepost.ru/) | Give the existing feed real pilot rides and albums. |

## My Garage and bike details

My Garage has a direct desktop link and a dedicated mobile Garage tab. The
existing routes and links remain available. The public Motorcycles catalogue
continues to serve discovery. The garage home should lead with each bike’s
cover, name, make, model, and year, with **Add motorcycle** easy to find.

Keep make, model, and model year as the required fields. Retain catalog search
and manual entry. Offer **Trim or edition** and **Color or livery** near the
identity fields, where owners can improve model and appearance accuracy.
The rest can sit under **More details**.

| Proposed details | How to use them |
| --- | --- |
| Nickname, trim or edition, color or livery | Personalize the card and identify the bike. Trim and paint are useful for illustrations. |
| Motorcycle category, engine displacement, transmission | Show a short specs row. Support touring, adventure, enduro, cruiser, naked, sport, scooters, and custom or other. Leave displacement blank where it does not apply. |
| Odometer in km, ownership status, owner notes | Keep these in the private garage. Current, project, and sold or archived are useful statuses for riders with several bikes. |
| Modifications and accessories | Start with a short owner-entered description. Consider structured parts and dates once pilot riders use it. |

The bike page can put the cover, identity, and short specs in **Overview**,
followed by **Photos and build** and the existing private **Service history**.
Keep sourced factory specs apart from owner-entered details. Preserve the
current public field set while the additional fields are piloted. Existing
upload visibility follows the bike. The owner can choose a cover from the gallery.

## One studio setting for generated bike covers

Use a landscape 3:2 composition for each verified bike variant. Frame the
entire motorcycle from the front left at axle height, with the front wheel
toward the left of the frame. Use a matte charcoal workshop, concrete floor,
and one broad overhead strip of soft light. Floor contact, space around the
wheels and mirrors, and the same camera position matter more than decoration.
Yamaha's studio photos and Ducati's authored configurator environments give
useful visual references for showing different bikes in a consistent setting.
[Yamaha brochure](https://www.yamaha-motor.eu/content/dam/yme/language-masters/en/eicma-2025/digital-brochures/2026_EICMA_MC_BROCHURE_EN.pdf) · [Ducati configurator](https://mediahouse.ducati.com/the-new-ducati-configurator-is-online-choosing-and-customizing-the-bike-of-your-dreams-has-never-been-so-engaging-358350/?lang=eng)

Test the [Image API edits endpoint](https://developers.openai.com/api/reference/resources/images/methods/edit)
with GPT Image 2.6 Sunburst at high quality first. Send a cleared reference
for the exact model, year, and trim and an original BIKERS.KZ studio image
as a separate setting reference. An edit can still alter the model's shape.
If the scene must match pixel for pixel, request a transparent cutout and
composite it over a single approved background with a fixed floor contact line
and shadow. Reuse the existing catalogue's verified model identity and check
permission to adapt each reference before sending it to the API. [OpenAI image generation](https://developers.openai.com/api/docs/guides/image-generation) · [OpenAI image prompting](https://developers.openai.com/api/docs/guides/image-prompting) · [Current model references](model-reference-images.md)

### Draft prompt

```text
Create a realistic 1536 × 1024 BIKERS.KZ garage cover for one
{year} {make} {model} {trim} in {color_or_reference_livery}.

Use reference 1 for the motorcycle. Match its exact silhouette, headlight,
fuel tank, fairings, frame, wheels, brake layout and exhaust. Preserve the
reference livery unless a verified factory color is provided. Keep all
factory equipment for this trim. Omit additions absent from the reference.

Use reference 2 for the setting, camera and light. Put the complete bike on
the concrete floor in the charcoal workshop. Shoot from the front left at
axle height, with the front wheel on the left. Use the same floor line and
soft overhead strip. Keep both wheels, mirrors and tail inside frame with
about 10% clear margin. Give the tires a realistic contact shadow.

Exclude people, license plates, invented decals, captions and watermarks.
```

### Pilot and delivery

1. Review ten verified models, including a CFMOTO 700MT, a KTM 390 Adventure,
   a Yamaha MT-07, and an older import. Compare Sunburst and Flare with the
   same references. Check geometry and trim against the source image and
   record time, metered cost, and review outcome. Test a freeform unknown
   model to confirm it keeps the standard fallback.
2. Save the bike immediately and start an image job after save. Show the
   existing cover or placeholder while it runs. Reuse an approved render
   keyed by catalogue model, year, trim, color, and scene version. Deduplicate
   jobs for the same key and discard stale results if the bike identity changes.
   A failed job or exhausted budget leaves the bike usable with its fallback.
3. Let the owner's explicitly selected photo win. Offer a reviewed generated
   cover labeled **AI illustration**, followed by the existing verified reference
   and a neutral card where neither is available. Keep generated assets
   separate from the verified reference catalogue. If an owner opts in to use
   their own photo as a reference later, keep that output scoped to their bike.
   Send only the necessary model and appearance fields. Exclude account data,
   VIN, plates and private notes.

For a 1536 × 1024 image, OpenAI's 2.6 calculator estimates about $0.020 at
low, $0.034 at medium, and $0.041 at high for the output alone. As an
illustration, 400 text tokens and 2,000 image input tokens add $0.018 to a
request. Twenty high-quality attempts with those inputs cost about $1.18.
With those inputs, $100 covers about 1,690 attempts before retries
or taxes. Actual image input sizes and usage vary. Set a $20 application-side
pilot cap, track actual cost per approved image, and confirm the credit grant
is attached to the API organization used for the pilot. [OpenAI cost calculator](https://developers.openai.com/api/docs/guides/image-generation#calculating-costs) · [OpenAI pricing](https://developers.openai.com/api/docs/pricing) · [API credits](https://help.openai.com/en/articles/20001626-api-credits-benefit-for-eligible-chatgpt-subscriptions)
