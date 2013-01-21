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
    keys = for word in words
      if word.length <= 2 then word else word[0..1]
    _(_.intersection(@data.b[key] for key in keys...))
    .filter((i) => @data.a[i][1].match(wordPattern))
    .first(100)
    .map((i) => @data.a[i])
    .value()

window.vcs = new Index

window.search = (e) ->
  val = e.target.value
  if val?.length > 0
    res = vcs.search(val)
  else
    res = []

  if res.length > 0 && e.keyCode == 13 # enter key
    document.location.pathname = "/firms/#{res[0][0]}.html"
  else
    res = _.map res, (vc) -> "<li><a href='/firms/#{vc[0]}.html'>#{vc[1]}</a></li>"
    document.getElementById("search-results").innerHTML = res.join("")
