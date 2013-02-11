class Index
  constructor: ->
    reqwest
      url: "/index.json"
      type: "json"
      success: (res) => @data = res

  search: (str) ->
    words = str.toLowerCase().match(/[a-z0-9]+/g)
    return [] if words == []
    wordPattern = new RegExp("\\b" + words.join("(.*\\b)+"), "i")
    keys = word[0] for word in words
    _(_.intersection(@data.b[key] for key in keys...))
    .filter((i) => @data.a[i][1].match(wordPattern))
    .first(100)
    .map((i) => @data.a[i])
    .value()

vcs = new Index

window.search = (e) ->
  val = e.target.value
  if val?.length > 0
    res = vcs.search(val)
  else
    res = []

  if res.length > 0 && e.keyCode == 13 # enter key
    document.location.pathname = "/firms/#{res[0][0]}.html"
  else
    res = _.map res, (vc) -> "<li><a onkeydown='arrow(event)' href='/firms/#{vc[0]}.html'>#{vc[1]}</a></li>"
    document.getElementById("search-results").innerHTML = res.join("")

  if e.keyCode == 40 # down arrow
    e.preventDefault()
    if li = document.getElementById("search-results").firstChild
      li.firstChild.focus()
    return false

window.arrow = (e) ->
  if e.keyCode == 38 or e.keyCode == 75 # up / k
    e.preventDefault()
    if prev = e.target.parentNode.previousSibling
      prev.firstChild.focus()
    else
      document.getElementById("search").focus()
    return false
  else if e.keyCode == 40 or e.keyCode == 74 # down / j
    e.preventDefault()
    if next = e.target.parentNode.nextSibling
      next.firstChild.focus()
    return false
