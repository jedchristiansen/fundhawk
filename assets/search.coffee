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
  val = e.srcElement.value
  if val?.length > 0
    res = _.map vcs.search(val), (vc) -> "<li><a href='/firms/#{vc[0]}.html'>#{vc[1]}</a></li>"
  else
    res = []
  document.getElementById("search-results").innerHTML = res.join("")
