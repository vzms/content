# VZMS Notes

## Rough Idea

- io.FS for pages and parts
- contents of a page/part:
  - map[string]interface{} for metadata (needs a doc page to outline what the properties are, e.g. page title, meta description, display title, various SEO tags, etc., etc. - these should be listed out somewhere for consistency but otherwise they are not hardcoded, just keys in a map - this is important because the list continues to grow and evolve and sites need to customize stuff, etc.)
  - map[string]struct{Content, ContentType}
  - methods:
    - get list of sections (may list more than the map above because some may not be editable - e.g. a comments section is a dynamic thing and it could have some settings in the metadata but no content)
    - render section - provide some render context (minimally it would need a URL) and returns (content, contentType, error)
  - nail down file format
    - preserve comments in metadata
    - multiple sections - decide if that's just top level HTML tags with IDs or what, maybe vzms-section= attribute
- Vue and Vugu UIs
- editor deals with filesystem concept
- but page rendering is just a handler that answers "give me all of the stuff I need on this page" - what goes in each section
- we don't need to do server-side rendering - we only need the TEXT CONTENT rendered.  this could be a major deal - we can provide a stripped down page render that handles the SEO concerns (we should give this a nice name like "shadow pages" or "fallback page" or something) but then otherwise just do real coding in the UI with Vue or Vugu.  Fallback content could be rendered with simple Go templates.
- and then "blocks" just provide a mapping of parts and their sections to named sections in a page
  - needs something for sequence

## Basic Design

Core concept: There is a Vue or Vugu app which performs page rendering in the browser.  It hits an API endpoint to collect up the various things that fit into "sections" on the page (e.g. header, footer, sidebar, main content, etc.).  The "editor" feature will be built into this Vue or Vugu app (I'm thinking we start with Vue but eventually write both) and will deal with separate editor endpoints to manage these various sections.

### Pages and Parts

A "Part" and a "Page" each refer to the same basic type (I'm thinking we call this "P").  This type is declared as:

```go
package p

// P describes a Page or Part.
type P interface {

	// get general data associated with this P
	Metadata() map[string]interface{} 

	// assign it
	SetMetadata(v map[string]interface{})

	// render the content to it's default content type (text/html for now but later
	// it could produce, e.g. markdown and we convert it to HTML after)
	RenderSection(rctx RenderContext, name string) (content []byte, contentType string, err error)

	// returns the raw content meant for editing - this may or may not be the same as produced
	// by RenderSection, returns nil content if no such section
	Section(name string) (content []byte, contentType string)

	SetSection(name string, content []byte, contentType string) error
}

// RenderContext has things a P can know about it's environment when rendering
// TODO: how to handle session
type RenderContext interface {
	URL() string
}
```

The only difference between a "Page" and a "Part" is that pages are accessible via their URL directly and parts are not. I.e. pages are something intended to be rendered directly at the URL that corresponds to the file (via some basic mapping, such as taking a URL path like `/content/blah` and appending `.vzht` to get `/content/blah.vzht`).  (Explanation of VZHT files is below.)  But parts might be internally at a path like `/parts/footers/default.vzht`, and the only way to use that is via the blocks system (see more on this below in the rendering and blocks sections).

### Filesystem

It's important to understand the `io/fs` package and fs.FS type, and the interface upgrade mechanism.  The basic idea is that `fs.FS` only provides the interface for read-only filesystem.  However, it is totally workable to provide other interfaces on top of this and use Go type assertions to check if more functionality is available.  Writing to a file is a good example.  I wish they had provided this functionality in the io/fs package, but whatever, we can still do it and it demonstrates the idea.

The [FS](https://pkg.go.dev/io/fs#FS) type is defined as simply:
```go
type FS interface {
	Open(name string) (File, error)
}
```

This allows us to open a file in read-only mode, not for writing.

However, we can make another interface that supports opening a file with different modes, which we can use to open a file for writing:

```go
type OpenFileFS interface {
	io.FS // inherit Open(string)(fs.File,error)
	// and support open with modes
	OpenFile(name string, flag int, perm fs.FileMode) (*fs.File, error)
}
```

Calling code that has a reference to a fs.FS can easily check for OpenFile support and use it if present:

```go
var fsys fs.FS := getAnFSFromSomewhere()
wfsys, ok := fsys.(OpenFileFS)
if !ok {
	return fmt.Errorf("filesystem %t does not support OpenFile", fsys)
}
f, err := wfsys.OpenFile("/some/path", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
// ...
```

This is the same idea we use for `PFile` shown below, except on the fs.File that is returned instead of the FS.  We provide additional methods that the caller can just check for via interface and use.

The intention here is to keep the interfaces as consistent as possible - so any tooling that knows how to walk a filesystem or do other such things will work seamlessly with our stuff, and we just add more interfaces/methods where needed. It takes a little getting used to but it's the recommended approach per the io/fs package and it's pretty easy to work with once you understand the pattern.

#### fs.FS Implementation

We'll need an implementation of `io/fs.FS` which also has an `OpenFile` method as above, and supports `PFile` on it's `fs.File` implementation.

It should probably support working on top of either an underlying OS filesystem, or any filesystem that provides an OpenFile method. This would allow other crazy scenarios in the future.  It also is convenient for testing where you can use something like https://github.com/psanford/memfs to implement an in-memory filesystem instead of having to manage temporary files.  Unfortunately that package's FS implementation does not provide an OpenFile method, so maybe just copy it and add that and include in our project.

Pages and Parts that are backed by files have the same format/structure (and implementation), simply because they are not different enough to warrant separate implementations.

#### Text File Format

For the current implementation, the content type will always be "text/html" and will live in a file with a '.vzht' extension (I'm open to suggestions if you don't like that). We can get into markdown and other stuff later.

It contains metadata as TOML at the top, followed by `---` a separator, then HTML with each section indicated with a simple HTML attribute (`vzms-section=`).

Example:

```
# this part is TOML
title = "Some Page"
meta_description = "This is just great"

# we use this as a separator before the HTML part ("---" is not valid TOML so it works and is easy to remember)

---

<!-- each section is just a tag with a vzms-section attribute, like so -->

<div vzms-section="main">
	This is the main content of the page
</div>

<div vzms-section="footer_additional">
	This is some additional stuff down near the footer
</div>

```

Note that the file format is tightly coupled to the fs.FS implementation, but it is not mandated. We can use an interface here and the fs.FS upgrade mechanism to express the abstraction:

```go
// PFile is implemented by filesystems that know how to convert from whatever underlying
// storage to a P (see package p).
type PFile interface {

	// reads a P from the underlying file,
	// only returns error if contents are invalid, otherwise 
	// empty file should return empty p.P 
	// (FIXME, that or we need some other explicit means of creating a new fresh p.P
	// with the appropriate implementation)
	ReadP() (p.P, error) 

	// writes a P to the underlying file, note that the implementation may
	// require that p.P was produced by ReadP() above (i.e. the actual underlying
	// p.P implementation was created from the same code)
	WriteP(p.P) error
}
```

The fs.FS implementation would then provide ReadP and WriteP methods on it's "File" type, so callers could cast to a PFile and ReadP would handle parsing the data and WriteP would handle encoding it.   Example, following from earlier one:

```go
var fsys fs.FS := getAnFSFromSomewhere()
wfsys, ok := fsys.(OpenFileFS)
if !ok {
	return fmt.Errorf("filesystem %t does not support OpenFile", fsys)
}
f, err := wfsys.OpenFile("/some/path", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
if err != nil {
	return err
}
defer f.Close()

pf, ok := f.(PFile)
if !ok {
	return fmt.Errorf("file %t does not support PFile methods", f)
}
p, err := pf.ReadP()

// ... make some changes to p ...

err = pf.WriteP(p)

// ...
```

NOTE: Comments in the TOML should be preserve as much as possible.  I'm thinking a simple approach would be to encode a comment above the `title` into the key `title__comment` or similar.  That way it's real easy to preserve them and they shouldn't get the way (and UI's can easily see what's a comment and display accordingly).

#### Database Table Implementation

TODO: We'll get into this later, but we absolutely should be able to implement the above on top of database tables.  We just don't want to do that right away for the initial MVP - instead let's begin by focusing on small sites that operate with the local file system only.

#### Custom Implementations

Note that there will also be custom implementations of the above that are not backed by files on the filesystem.  The main example I can think of is dynamic things on the page (comments section, automatic lists of links, generated from looking at another filesystem, etc.) where the `RenderSection` method is code that is doing something specific.  In this case, the `Section` and `SetSection` methods would probably return some sort of "not implemented" error.  So in an UI editor, these things would show up and be available to configure and use on pages, but the editor would error or have a disabled button, etc. if you were to try to use the WYSIWYG editor on them (although a read-only preview could still be implemented by calling `RenderSection`, just like the regular page does).

### Docs for Meta and Section Names

Both metadata key names and section names should be each clearly documented with a standard list of names somewhere easy to get to in the VZMS docs.  These names cannot be enforced entirely in code because applications always need to customize and add more, omit others, etc.  All that is really needed as a much CONSISTENCY as possible between different templates and plugins, etc. so that people use the same names for the same things as much as possible.  This is a documentation problem, not necessarily a code problem.  In the code these are just strings.  But which strings people use we can only guide through documentation and showing people best practices.  Examples of some of these:

#### Section Names

- `main` - primary contents in the "main" part of the page
- `header` - section above main
- `top_nav` - section with navigation above main and below header
- `footer` - section below main
- `left` - section to the left of main
- `right` - section to the right of main

NOTE: There will definitely be variation in these names in sites out in the wild. Some templates will have strange sections that need new names or omit other sections because they don't make sense - that's completely fine.  The goal is not 100% compatability (users will need to configure these mappings anyway in plenty of cases). The goal is that BY DEFAULT the names should match so things "just show up in the right place" wherever feasible.  E.g. you enable some section that is supposed to provide navigation and it just shows up in `top_nav` by default.  You can configure this, but the default is what you would expect.

#### Metadata Keys

- `title` the contents of the &lt;title> HTML tag
- `meta_description` META description tag
- `main_title` the title that shows in the &lt;H1> tag or similar, if not present suggested behavior is to read `title` instead

TODO: There are a fair number of SEO tags that often need to be set, e.g. see https://ogp.me/ and https://schema.org/ - whatever is applicable there should appear here with as direct and mapping as possible.

### Rendering

From the persecptive of the Vue/Vugu code that is rendering a page, it will ask the CMS via an API call to "give me all of the sections and metadata I should show on this page".  And then that code will use the metadata as appropriate (e.g. change the title tag) and take the various named sections and fill them in.  Here's a rough idea of how this would go:

Endpoint: `/api/page-render`

Params:

- `page_path` - the page path to render, from the URL, so if the Vue app were navigating to /blog/some-page then that would be passed here
TODO: any others?

The algorithm the endpoint would follow would be:
- file the path indicated by `page_path` by asking the FS to load it
- extract metdata
- render each section
- consult the blocks system to find additional sections (see more on Blocks below)
- merge it together and output

And then it's results would be returned as JSON like so:
```json
{
	"metadata": {
		// properties extracted from TOML section of page
		"title": "The Page Title Here"
	},

	"sections": {
		"main": {
			"content": "<div>Some main content here<div>"
		},
		"footer": {
			"content": "<div>Footer markup goes here<div>"
		}
		// etc
	}
}
```

TODO: need multiple

NOTE: that path prefixes are totally fine and we can do provide for that.  E.g. you can map `/pages/*` to whatever underlying fs.FS is providing the pages for that URL section, etc.  But also realize that you don't have to - you could also map `/` and have it check all URLs and just fall through to the next thing to check for the case where nothing is found.  This mapping would just be done statically in the writing in main.go

### Editing

The rendering stuff above is a read-only procedure specifically for getting content onto a page.  Editing is different.  The idea would be the user clicks some sort of "edit this page" button in the UI and the Vue code goes into Edit Mode.  In this mode it consults a different set of API calls to deal with manipulating page and part data.  The "page" editing would be done via endpoints something like:

GET `/api/page?page_path=` - returns the contents of the `P` but as JSON. The format is very similar to the rendered result, but it only contains data from this one single page (it corresponds directly to a VZHT file without any merging with blocks, etc.), and it also has the content types, because we could suppor things like editing markdown, etc., whereas the render step would convert this to final HTML and always return HTML content. It would look something like:

```json
{
	"metadata": {
		// properties extracted from TOML section of this page
		"title": "The Page Title Here",
		// including comments
		"title__comment": "I like that comments are preserved during editing, it makes me feel good"
	},

	"sections": {
		"main": {
			"content": "<div>Some main content here<div>",
			"content_type": "text/html"
		},
		"footer": {
			"content": "<div>Footer markup goes here<div>",
			"content_type": "text/html"
		}
		// etc
	}
}
```

And PUT `/api/page?page_path=` would do the opposite and write the result, same JSON format

POST would add a new one

And DELETE would remove one

GET `api/page-list` could be used to get a list of pages.  It probably needs to include a means of iterating over large sets, but we might want to delay that problem until we see how it would work in the UI (iteration over large sets is dependent on the use case, I have experience with this and there are multiple approaches).

And then "parts" would be editable only via the Blocks API, see below.

### Templates

The "template" aspect of different layouts and CSS, etc. is basically the problem of the Vue or Vugu application.  We'll need to organize this later, but essentially it has nothing to do with content management itself - it's just a detail of how the page is rendered, and encapsulated by the UI code.

### Blocks

The purpose of the Blocks system is to provide a mapping of random Parts to URL/path patterns so they can show up on various pages.  Examples of use cases:

- static content like featured articles that should just show up on certain pages in a section on the side
- dynamic lists of things where the block mapping says where to show up, but the actual underlying content is dynamically created (i.e. the `RenderSection` method is implemented as raw Go code and not backed by just HTML in a file)

So that would give us multiple fs.FS implementations.  One that would look at a folder on disk and return `P` instances (just like pages), but then each other dynamic thing that needs custom code would have it's own implementation.

The relevant interface methods are the same thing here as mentioned earlier.  Implementations that support only reading would implement an interface on the fs.File they return like:

```go
// PFileReader would be the interface for things that provide blocks that are only dynamic and driven by code, not editable.
type PFileReader interface {
	ReadP() (p.P, error) 
}
```

And anything that supports editing would be the same interface as described earlier:
```go
type PFile interface {
	ReadP() (p.P, error) 
	WriteP(p.P) error
}

```

The pattern should be starting to look familiar now.

That's the internal aspect of editing Part contents.   The other task is dealing with how they map to which URLs they show up on and providing a facility to edit this.

#### Block Path Mapping

We'll need some basic configuration

```go
package blocks

// BlockMapper basically manages Selectors and provides a method to match a URL
type BlockMapper interface {
	SelectorList() ([]Selector, error)
	SetSelector(index int, s Selector) error
	DeleteSelector(index int) error
	AddSelector() Selector

	Match(url string) map[string]struct{Content,ContentType string} // TODO: this should be it's own type some place
}

type Selector struct {

	Name string // name for humans, can be autogenerated or UUID text if it doesn't matter

	PartPath string // the path to read the part from

	// the section name in the Part, if empty it matches all of them
	FromSection string 

	// the section name to output to in render result, if empty then FromSection is used,
	// providing a ToSection without a FromSection is not valid
	ToSection string

	// the rules to match - these are interpreted as an OR, so any of these that 
	// match will cause the selector to fire
	Rules []Rule

}

type Rule struct {

	// I think we want to leave this really generic, so the frontend and backend can stay
	// compatible but evolve separately.
	// Examples of MatchType values could be:
	// "path_eq" - means MatchCriteria is an exact URL path
	// "path_glob" - means MatchCriteria is a glob expression, e.g. "/pages/*"
	// "path_regexp" - means MatchCriteria is a regular expression, e.g. "^/pages/[^/]+/xyz$"
	// "!path_eq" - matches the opposite of path_eq
	// and so on
	MatchType string

	// value dependent upon MatchType
	MatchCriteria string

	// TODO:
	// And []Rule
	// Or []Rule
	// Allows for the case where you want to perform more complex matching - it will
	// eventually be needed but not necessarily MVP
	
	// The idea here is that the UI can backend can evolve separately.  If a new MatchType
	// is implemented in the backend but not in the UI, the UI could just show the raw data
	// in these fields so at least things are still editablabe, just less fancy/explanatory
	// (because the UI doesn't know what the new match type means).  New fields could just
	// be preserved by the UI and not touched.  Better than a save breaking things.
	// Conversely, if the UI sends something the backend doesn't understand/support, the backend
	// can error when SetSelector is called and that error can get propagated up to the UI.
}
```

This is what the `api/page-render` URL would interact with in order to find matching sections.

And then there should be an implementation for this which operates on a TOML file:

```toml

[selectors.name1]

part_path = "/path/to/part.vzht"
from_section = "left_nav"
to_section = "right_nav"
rules = [ { match_type="path_eq", match_critera="/some/page.html" } ]

[selectors.name2]
# ...

```

And then we'll need a handler to expose this to the Vue/Vugu code that allows editing this in a UI:

GET `/api/blocks` - lists the selectors
GET `/api/blocks/{name}` - get one selector
PUT `/api/blocks/{name}` - write an individual selector, JSON is equivalent names and structure to GO and TOML above
Same pattern for POST and DELETE

The editor would use these endpoints to provide a nice UI to map which things show into what slots on various pages, etc.  Also things like when you just drag a block to some section - it could write out a "path_eq" selector so it just shows on that one page and then has a subtle prompt to say "you've assigned this block to this page, would you like to configure which other pages it shows up on?" and pop open the full editor if they click it, etc.

### Fallback Rendering

I think this feature will dramatically reduce complexity in the system.  Instead of trying to render the same page on server side and in the UI in JS (Vue, React, etc.) or WASM (Vugu), we just render a basic page that blorps out each of the matching sections.  Under the hook it would be calling the same code as `/api/page-render` to match the various sections, etc.  But instead of trying to output into the nice fancy template with all of the dynamic features, etc., instead it just dumps out the contents into a generic template that doesn't necessarily need to look nice or have all of the same features - no UI editing, etc.  It still would be editable by developers, but it's just separate.

This could be based on simple Go templates.  I think this is best because of familiarity and how widely used it is.  We really don't need much for this.  The main point is to provide the text for SEO with some basic styling without complicating the main UI in JS and/or WASM.
