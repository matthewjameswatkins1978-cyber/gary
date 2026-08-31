

You are GARY, the automated build and verification system.
Build Book Burner, a serious native iPhone application for rapidly turning a physical pile of books into individually identified, photographed, located and exportable inventory.
This is not a prototype exercise and not an architecture demonstration.
The first usable release must already solve the real problem.
Do not stop after:
- project scaffolding
- a scanner demo
- a database model
- a pretty UI
- mocked metadata
- simulator-only success
The first useful build must let Matthew stand beside a pile of real books with an iPhone and process them rapidly.
Use professional Swift/iOS engineering standards throughout.
Book Burner is not a library or reading-management application.
It is a book capture + liquidation workflow.
The central user experience is:
scan ISBN → photograph actual front cover → auto-save → immediately scan next book
Everything else exists to support that loop.
The user should not normally:
- type
- press Save
- choose condition from forms
- research prices while scanning
- switch applications
- wait for metadata
- resolve problematic books before continuing
Failures must leave evidence, not block the conveyor belt.
This is initially a single-user personal application.
The user has:
- an iPhone as the authoritative capture device
- a large physical book collection
- books that may need to be sold, bundled, donated or kept
- duplicate books
- books without readable barcodes
- intermittent lookup failures
- boxes/shelves/locations that must be remembered
Optimise for actual rapid physical use.
Do not optimise 0.1 for theoretical multi-user SaaS requirements.
Call this:
Book Burner 0.1 Capture Edition
0.1 is not complete unless all of the following work together.
1. Scan an ISBN/barcode.
2. Normalise and validate the ISBN where possible.
3. Allocate a permanent Book Burner identifier:
   - BB-000001
   - BB-000002
   - etc.
4. Start metadata enrichment automatically.
5. Prompt visually to flip/show the front of the book.
6. Capture a real photograph of that physical copy.
7. Persist the copy locally.
8. Give success haptic/audio feedback.
9. Return immediately to scanning.
10. No Save button.
The app must also support:
- duplicate ISBNs
- lookup failure
- offline capture
- no ISBN
- unreadable barcode
- incorrect metadata match
- interrupted/incomplete capture
- photo failure
- app restart during an unfinished capture
None may destroy already captured information.
ISBN is not the primary identity of a book copy.
Two physical copies with the same ISBN are two different Book Burner records.
Each physical copy receives a permanent BB identifier.
Use a robust internal identity such as UUID if appropriate, but expose a permanent human-readable BB ID as a first-class domain identifier.
BB numbers:
- must be unique
- must increase monotonically
- must survive application restarts
- must never collide
- should not be reused after deletion/Undo
Test this explicitly.
Build this as an experienced professional Swift engineer would.
Use:
- current stable Xcode available on the build Mac
- current stable Swift language mode
- current stable iOS SDK
- SwiftUI as the primary UI framework
- SwiftData for persistence unless testing exposes a concrete blocker
- VisionKit/DataScannerViewController for barcode scanning unless a measured limitation requires another implementation
- Foundation URLSession networking
- modern Swift structured concurrency
- async/await
- proper actor/MainActor isolation
- Sendable correctness
- Swift Observation/state-management patterns appropriate to the current SDK
- native haptics
- native navigation
- native file sharing/export
Do not target a beta OS merely because it is newer.
Target an OS version that runs correctly on Matthew's actual iPhone.
Do not introduce third-party frameworks when an Apple framework does the job adequately.
Use feature-oriented architecture.
A sensible shape is:
BookBurner/     App/      Features/         Capture/         Books/         BookDetail/         Locations/         Export/      Domain/         Books/         Identification/         Metadata/         Research/      Infrastructure/         Persistence/         Networking/         Photos/         Archiving/      Integrations/         GoogleBooks/         OpenLibrary/         Pricing/      Shared/         UI/         Utilities/      Tests/
This is a guide, not a mandated directory fetish.
Prefer clarity over architectural ceremony.
Do not create:
- giant global service containers
- enormous view models
- meaningless one-method protocols
- unnecessary repositories around repositories
- Redux/TCA machinery without evidence that the app needs it
- bespoke dependency-injection frameworks
Use protocols at external/testable boundaries where they earn their existence.
Design this carefully before UI code spreads assumptions everywhere.
At minimum model:
Should contain or relate to:
- internal stable ID
- BB ID
- ISBN-13
- ISBN-10 where known
- title
- subtitle where known
- authors
- publisher
- publication date/year
- language
- page count where available
- metadata source/provenance
- source identifiers
- capture time
- update time
- location
- identification state
- metadata state
- research state
Do not force one giant status enum to represent unrelated dimensions if separate state fields produce a cleaner model.
Allow more than one photograph per book from the beginning.
Photo roles should support at least:
- front
- copyright page
- signature
- dust jacket
- printing line
- other
0.1 requires only the front photo in the normal path.
Simple named location, for example:
- Box 1
- Box 2
- Office Shelf
- Bedroom Shelf
- Unassigned
Do not build a warehouse-management hierarchy.
The app needs enough persistent state to recover if terminated during capture.
Do not put full-resolution image blobs into SwiftData unless evidence demonstrates that this is clearly superior.
Prefer app-managed image files with persisted relative references.
Use deterministic filenames where practical, for example:
BB-000127-front.heic BB-000127-copyright.heic
Image writes must be robust.
Prefer:
1. write temporary file
2. verify successful write
3. atomically move/rename into permanent location
4. commit corresponding persistent reference
Avoid orphaned image files and database rows pointing at nonexistent files.
Optimise front-cover images for:
- readable cover text
- visible copy condition
- edition clues
- practical storage size
Do not destroy useful detail merely to shave a few kilobytes.
Treat capture as an explicit workflow, not scattered booleans.
Conceptually:
readyToScan     ↓ barcodeLocked     ↓ awaitingFrontPhoto     ↓ saving     ↓ saved     ↓ readyToScan
Metadata enrichment should run independently.
A network request must not hold the capture workflow hostage.
If metadata is slow, the user should still be able to photograph and save the book.
If metadata fails, save:
- BB ID
- ISBN if known
- front photo
- location
- timestamp
- failure state
Then continue.
Do not reject duplicate ISBNs.
If the same ISBN is scanned five times, create five different BookCopy records with five BB IDs.
An unobtrusive note such as:
Another copy of this ISBN exists
is acceptable.
It must never become a blocking confirmation dialog.
Restrict live recognition to barcode types that actually matter for books rather than asking VisionKit to recognise everything.
Normal workflow:
1. scanner sees book barcode
2. ISBN extracted/normalised
3. capture state locks that scan
4. repeated camera events for the same physical barcode are ignored while that capture is active
5. after completed save, scanner resets
6. scanning the same ISBN again is allowed as another physical copy
Do not implement global ISBN deduplication.
Burn mode must contain an unobtrusive escape route such as:
No ISBN / Can't Scan
This must allow:
1. allocate BB ID
2. photograph front
3. inherit current location
4. save record
5. mark it for later identification
6. return to scanner
Do not require manual title entry at the pile.
Implement metadata behind a clean provider boundary.
Suggested conceptual interface:
BookMetadataProvider
Implement at least:
Search public book metadata using ISBN.
Google Books supports the isbn: query syntax. Public requests require appropriate application identification such as an API key.
Do not commit API credentials.
Use as fallback/additional metadata source.
Something along these lines:
Google Books     ↓ no useful match Open Library     ↓ no useful match Needs review
Do not make either provider's JSON structures part of Book Burner's domain model.
Use:
Provider DTO     ↓ mapping Book Burner domain model
Cache useful successful metadata by normalised ISBN where appropriate.
No credentials, tokens or API secrets in Git.
Provide:
Secrets.xcconfig.example
or another standard developer configuration mechanism.
Ignore real secret files in Git.
If a Google Books key is unavailable, the app should degrade intelligently, for example by using the other provider and leaving enrichment pending.
Do not make metadata credentials a condition for locally capturing books.
Physical capture outranks the network.
Internet unavailable:
scan → photograph → save locally → metadataState = pending → next book
When connectivity later returns, enrichment may be retried.
Do not require sophisticated background scheduling for 0.1 if simple deterministic retry on app activation/library access achieves the job reliably.
Avoid unnecessary complexity.
Current location is session-like state.
Display the current location clearly in Burn mode.
The user should be able to tap it and quickly:
- choose an existing location
- create a new location
Every subsequent captured book inherits that location until changed.
There should always be a safe fallback such as:
Unassigned
Lack of a location must not prevent capture.
Provide:
Undo Last Book
Undo should:
- remove the most recent capture safely
- remove associated photo assets appropriately
- not corrupt neighbouring records
- not reuse the BB number
Test Undo after:
- ordinary capture
- duplicate ISBN
- offline capture
- unresolved capture
This should look and behave like a confident modern native iPhone utility.
Not:
- an enterprise inventory system
- an HTML dashboard wrapped in Swift
- a fake wooden bookshelf
- a form-heavy catalogue
- a neon startup template
Use system typography and native controls.
Book photographs already provide the visual texture.
Prefer restraint.
For 0.1 prefer only two primary sections:
The high-speed camera workflow.
Captured inventory.
Search belongs naturally inside Books rather than requiring another whole navigation universe.
Use native TabView/navigation appropriate to the current stable SwiftUI SDK.
Camera dominates the screen.
Useful visible controls/information:
- current location
- current session/book count
- torch where supported
- no-ISBN path
- Undo last
After ISBN recognition, switch cleanly into front-photo state:
Flip to front
Then provide the obvious shutter action.
After successful save:
- positive haptic
- optional short sound
- tiny transient book confirmation
- automatically resume scanning
Avoid modal dialogs in the normal loop.
Use a native performant list.
A row should communicate roughly:
[actual front thumbnail]  Title Author · Year  BB-000127            Needs research
Possible state indicators:
- Ready
- Lookup pending
- Needs review
- No ISBN
- Wrong match
- Needs photo
Do not turn every attribute into a coloured badge.
Use native SwiftUI search.
Search useful fields including:
- BB ID
- title
- author
- ISBN
- publisher
- location
Provide lightweight filtering for things such as:
- unresolved
- lookup pending
- needs review
- location
Do not implement a query language.
One native vertically scrolling detail screen.
Include:
- large actual-copy photograph
- title
- authors
- BB ID
- ISBN
- publisher/date
- location
- status
- metadata/provenance
- additional photos when present
Actions can include:
- change location
- mark metadata incorrect
- request/add another photo
- delete
- later: research book
Do not bury core facts behind menus.
Treat accessibility as ordinary engineering.
Verify:
- Dynamic Type
- VoiceOver labels for controls
- sufficient contrast
- 44pt minimum interactive target where applicable
- Light Mode
- Dark Mode
- sensible Reduced Motion behaviour
- haptics not being the sole source of state information
Do not postpone this until "polish".
No fatalError, forced unwrap or crash should be reachable from ordinary user/device/network failure.
Handle at minimum:
- camera permission denied
- scanner unavailable
- photo capture failure
- disk write failure
- malformed API response
- HTTP timeout
- provider unavailable
- no metadata match
- invalid ISBN
- persistence failure
- export failure
Errors should explain what matters and offer a useful recovery action.
Use structured concurrency correctly.
Do not silence Swift concurrency warnings.
Network enrichment should be cancellable where sensible.
UI mutations belong on appropriate actor isolation.
Persistent shared state should have an explicit owner.
Avoid:
- detached tasks without reason
- races around BB ID allocation
- unstructured Task proliferation
- callbacks mixed randomly with async/await
Compile with strict concurrency checking appropriate to the current toolchain.
Treat local user data as valuable from the first release.
Use a versioned SwiftData schema.
Add migration tests.
Do not assume the database can simply be deleted when models change.
An upgrade from one shipped version must preserve captured books.
Export is not future work.
A user must be able to export the collection as:
BookBurner-YYYY-MM-DD.zip  books.csv books.json photos/
Strongly consider also:
manifest.json
containing:
- export schema version
- creation time
- app version
- record count
Use the system file exporter/share experience.
CSV must:
- be valid UTF-8
- escape quotes correctly
- escape commas correctly
- handle line breaks correctly
- include stable useful columns
- preserve BB IDs
- preserve ISBNs as text
- include relative photo references
- include location/state fields
Test nasty titles containing:
- commas
- quotes
- apostrophes
- Unicode
- Welsh characters
- line breaks
JSON is the canonical richer machine-readable export.
Include an explicit schema version.
Preserve:
- BB identity
- metadata
- states
- locations
- photo references
- provenance
- timestamps
Do not export arbitrary Swift implementation details.
The export format is a product interface.
Treat it accordingly.
First check whether the current target Apple SDK provides a satisfactory standards-compatible native archive route.
If not, use one small, well-maintained, appropriately licensed Swift Package for ZIP creation.
Pin it.
Wrap it behind Book Burner's own archive boundary.
Do not write a homemade ZIP implementation.
Do not add a kitchen-sink utility framework solely for compression.
Given N persisted books:
- CSV must contain N book rows
- JSON must contain N books
- every exported photo reference must resolve
- no unrelated temporary files should be included
- duplicate ISBN copies must remain distinct
- unresolved records must remain present
- export must open successfully outside the app
Tests should inspect the resulting archive mechanically.
Do not create hundreds of ceremonial tests.
Write high-value tests around domain correctness.
Mandatory automated coverage includes:
- ISBN normalisation
- ISBN-10/ISBN-13 validation
- BB ID generation
- BB IDs surviving restart
- BB IDs not reused
- duplicate ISBN copies
- metadata provider mapping
- provider fallback
- malformed metadata
- network failure
- offline pending state
- location inheritance
- location changes
- capture state transitions
- interrupted capture recovery
- Undo
- photo-file lifecycle
- CSV escaping
- JSON schema
- export completeness
- migration of persisted data
Use fixtures for provider responses.
Do not make routine unit tests contact live external APIs.
VisionKit behaviour itself cannot be fully simulated in ordinary unit tests.
Separate:
barcode acquisition
from:
what Book Burner does with a recognised barcode
Create a testable seam so the capture workflow can be driven by fake scanner events in tests.
Do not fake the final real-device test.
Simulator tests are useful but insufficient.
Camera/barcode acceptance requires a physical iPhone.
At minimum verify:
- first camera permission flow
- live barcode recognition
- repeat scanning
- same ISBN twice
- front-photo capture
- torch
- rotate/background/foreground behaviour as supported
- app interruption
- actual book pile ergonomics
- performance after many captures
Never report "real device passed" unless a real device actually ran it.
Prepare a physical acceptance script containing at least:
- ordinary modern ISBN book
- same ISBN twice
- book with damaged barcode
- book with no useful barcode
- book whose metadata is missing
- book deliberately scanned offline
- change location mid-session
- Undo one capture
- terminate/relaunch during incomplete capture
Then ultimately:
messy 50-book test
This is the release-quality test.
Matthew should be able to process approximately 50 genuinely mixed books without the app becoming irritating.
Optimise perceived capture latency.
The network should not determine how quickly the user can move to the next book.
Measure on the real target iPhone.
The goal is that after the front-photo shutter and successful local persistence, Book Burner returns to scan-ready state fast enough that the user is not waiting on the software.
Record timings during the real-world test rather than inventing benchmark victories.
Use Apple's appropriate logging facilities.
Useful diagnostic areas:
- capture
- scanner
- metadata
- persistence
- photo storage
- export
Logs must never contain API secrets.
Do not litter production code with print() debugging.
If a Book Burner repository already exists:
- inspect it
- report branch/SHA
- preserve useful existing work
- do not blindly replace it
If no canonical repository exists, create:
matthewjameswatkins1978-cyber/book-burner
or prepare the local Git repository for that canonical remote if repository creation is unavailable.
Suggested initial branch:
gary/book-burner-0.1
Make meaningful commits by completed capability.
Do not make one enormous "Book Burner done" commit.
iOS builds and authoritative Xcode tests must run on macOS with Xcode.
Do not pretend a Linux/Windows/cloud-only Swift build proves the iPhone application.
GARY may coordinate development elsewhere, but the authoritative app build must happen on Matthew's available Mac build machine.
Discover the actual Mac/Xcode environment rather than inventing filesystem paths.
If Mac access is unavailable:
- continue any platform-independent preparation that is genuinely useful
- clearly report the blocker
- do not claim the iOS build is verified
Use normal Apple development signing.
Do not request passwords or authentication secrets through source code/config.
If a one-time human action is genuinely required, such as:
- selecting development team
- accepting a signing prompt
- trusting the physical iPhone
stop at that exact boundary and ask one concise question.
Do not repeatedly interrupt for things that can safely be inferred.
GARY should break the master project into bounded jobs with mechanical acceptance.
Deliver:
- Git repo
- Xcode project
- app launches
- SwiftData model container
- initial domain model
- test target
- formatting/lint configuration
- secret configuration template
Acceptance:
- clean build
- unit tests pass
- app launches on simulator
- no committed secrets
Deliver:
- BookCopy
- BookPhoto
- StorageLocation
- BB ID allocator
- schema versioning
Acceptance:
- IDs survive restart
- duplicates allowed
- IDs not reused
- migration baseline exists
Deliver:
- VisionKit scanner
- ISBN recognition
- normalisation
- capture state lock
- scanner availability handling
Acceptance:
- real barcode scan on iPhone
- repeated detection doesn't duplicate one physical scan
- same ISBN can be deliberately captured again
Deliver:
scan → flip → photo → persist → next
Acceptance:
- actual image file saved
- BookPhoto relationship correct
- no Save button
- scanner returns automatically
At this point the application must already be minimally usable.
Deliver:
- Google Books adapter
- Open Library adapter
- DTO mapping
- provider fallback
- caching
- async enrichment
Acceptance:
- successful known ISBN lookup
- failure doesn't block capture
- offline capture works
- malformed response doesn't crash
Deliver:
- No ISBN
- barcode unreadable route
- lookup pending
- wrong match
- needs review
- incomplete recovery
Acceptance:
- every failure produces a recoverable record
- user can continue immediately
Deliver:
- create/select current location
- inherited location
- Unassigned fallback
- Undo Last Book
Acceptance:
- location changes affect only subsequent captures
- Undo removes correct record/assets
- ID never reused
Deliver:
- list
- detail
- search
- filters
- unresolved visibility
- wrong-match action
Acceptance:
- 100+ fixture books scroll/search cleanly
- accessibility checks pass
Deliver:
- CSV
- JSON
- photos
- ZIP
- native exporter
Acceptance:
- mechanical archive inspection passes
- Unicode/quotes/commas handled
- duplicate/unresolved books preserved
Run realistic device scenarios.
Then run a 25-book test.
Repair friction.
Then run the full approximately 50-book test.
Only after that call 0.1 trustworthy.
Do this after the complete workflow exists.
Improve:
- camera transitions
- haptic hierarchy
- shutter feedback
- transient saved card
- list typography
- location picker
- empty states
- permission recovery
- loading behaviour
- Dark Mode
- Dynamic Type
Do not spend early jobs drawing decorative screens while export or failure recovery is missing.
Explicitly out of scope:
- eBay listing creation
- eBay account management
- Amazon seller integration
- CloudKit sync
- custom server
- user accounts
- subscriptions
- AI vision identification
- OCR edition identification
- automatic sold-price scraping
- shipping calculator
- detailed condition questionnaire
- dashboards
- business analytics
- inventory charts
- multi-user collaboration
- complex warehouse locations
- fancy cover-art downloads as a substitute for actual-copy photography
Do not smuggle these in as "future proofing".
After 0.1 capture/export is trustworthy, build market triage.
This must not delay 0.1.
The pricing system should distinguish:
asking-market evidence actual-sale evidence valuation estimate confidence
Never call a current asking price a sold price.
Never infer a precise value from one listing.
The official eBay Browse API can search current inventory using GTIN/ISBN.
Use it for active asking-market evidence.
Prefer:
1. ISBN/GTIN
2. exact edition clues where available
3. title/author fallback when necessary
Capture raw comparable observations separately from interpreted valuation.
Conceptually:
ListingObservation     ↓ PriceSignal     ↓ ValuationEstimate
Do not flatten these into one number.
eBay Browse API uses application authentication.
Do not embed an eBay client secret in the iPhone binary.
When 0.2 is implemented, choose one of:
1. a minimal secure server-side pricing gateway holding the eBay credential; or
2. an external Lucy/GARY research workflow triggered by Book Burner.
Prefer the smallest justified solution.
If a backend becomes necessary, it should have one narrow purpose, not become a Book Burner cloud platform.
For matching active listings calculate robust signals such as:
- match count
- minimum plausible asking price
- median
- interquartile range or similar spread
- postage where available
- condition distribution
- edition-match quality
Reject or down-weight obvious outliers.
Do not output fake precision such as:
£23.47
Prefer:
£18–£30 asking market
Medium confidence
with evidence:
16 plausible matches
Interesting books should support:
Research this book
Create a portable research packet containing:
- BB ID
- ISBN
- title
- authors
- publisher
- publication information
- metadata source
- photos
- known price signals
This can be handed to Lucy/GARY/a browser agent.
The research system may then investigate:
- actual sold evidence where legitimately accessible
- specialist booksellers
- AbeBooks/Biblio-type listings
- auction results
- rare edition information
- signatures
- printing lines
- specialist marketplaces
Do not build brittle HTML scrapers into the iOS client.
A research result may request additional physical evidence such as:
- copyright page
- printing line
- signature
- dust jacket
- title page
Book Burner should later present these as small physical-photo jobs linked to BB ID.
The capture architecture must therefore support multiple photo roles from the beginning.
Later valuation can broadly classify inventory into buckets, for example:
ordinary / low value worth bundling worth individual sale research further potentially valuable unknown
Do not spend expensive research compute on obviously ordinary copies.
The product principle is:
cheap triage first, deep research only where uncertainty has economic value.
0.1 is done when all of these are true:
- real iPhone build installed
- barcode scan works
- BB IDs work
- duplicate ISBN works
- metadata works
- offline capture works
- front photos work
- auto-save/next works
- no-ISBN path works
- lookup failure doesn't block
- locations work
- Undo works
- library browsing works
- unresolved queue/filter works
- CSV works
- JSON works
- photo export works
- ZIP works
- migration tests pass
- unit/integration tests pass
- no secrets committed
- no material Swift concurrency warnings
- real 25-book test passed
- real messy approximately 50-book test passed or is awaiting Matthew's physical acceptance
If the final physical test has not happened, report:
CODE COMPLETE, PHYSICAL ACCEPTANCE PENDING
Do not call it completely verified.
Use GARY's normal bounded-job discipline.
For every worker job provide:
- exact goal
- permitted scope
- relevant files
- acceptance tests
- evidence required
- explicit non-goals
Prefer small jobs roughly in the normal GARY work slice.
Long-running builds/tests may exceed the slice when mechanically justified.
Use stronger reasoning workers for:
- domain architecture
- Swift concurrency
- scanner lifecycle
- persistence/migrations
- final review
Cheaper workers are suitable for:
- fixtures
- straightforward tests
- documentation
- mechanical UI cleanup
Use Telltail/GARY's normal acceptance machinery where available.
Model self-report is not proof.
When a gate fails:
1. preserve failure evidence
2. identify the smallest responsible layer
3. issue a bounded repair job
4. rerun the failed test
5. rerun nearby regression tests
6. continue only after mechanical acceptance
Do not rewrite the application because a single adapter or screen is broken.
Infer ordinary implementation decisions.
Ask Matthew only when a decision genuinely needs him, particularly:
- Apple signing/account action
- physical device interaction
- choice involving money
- external account creation requiring consent
- destructive replacement of existing project work
- two materially different product behaviours where intent is genuinely ambiguous
Do not ask him which Swift collection type to use.
At the end of each major milestone report:
BOOK BURNER STATUS  Branch: Commit: Xcode: Swift: Target iOS: Device tested:  Passed: Failed: Pending human/device action:  What now works: Known limitations: Next gate:
At 0.1 completion also provide:
- install/build instructions
- test command
- architecture summary
- export schema description
- API/configuration setup
- real-device test results
- known limitations
- Git tag
Suggested tag:
book-burner-v0.1.0
When choosing between:
more architecture
and
a faster, reliable book going through the conveyor
choose the reliable book.
When choosing between:
a shortcut that will corrupt/strand user data
and
proper engineering
choose proper engineering.
Book Burner should be small, native, fast and trustworthy.
Build the application Matthew can actually use, not the application an architecture diagram would like to describe.
That’s the job I’d give GARY. It gives it room to make competent Swift decisions, but removes the dangerous freedom to redefine “first version” into a scanner prototype.
The other important change is the eBay boundary. A serious coder shouldn’t put an eBay application secret inside an iPhone app, so I’ve deliberately moved authenticated automatic market pricing behind 0.1 and made GARY choose a narrow server-side or external research path when we get there. The official Browse API can search active listings by ISBN/GTIN, but it requires an application access token, so that separation is technically justified rather than merely cautious. [eBay Developers](https://developer.ebay.com/develop/api/buy/browse_api?utm_source=chatgpt.com)
And 0.1 remains useful without it: you can go to the pile, scan fifty books, photograph them, know where every copy is, survive all the awkward ones, and hand me a clean export. Then we bolt the clever valuation machinery onto something that already earns its keep.