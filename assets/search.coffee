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

if document.location.pathname == '/' || document.location.pathname == '/index.html'
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

# From multitrack (https://github.com/drchiu/multitrack/blob/master/templates/visit.js.erb)
uniqueId = ->
  'xxxxxxxx-xxxx-4xxx-yxxx'.replace(/[xy]/g, (c) ->
    r = Math.random()*16|0
    v = if c == 'x' then r else r&0x3|0x8
    v.toString(16)
  ).toUpperCase()

readCookie = (cookieName) ->
  theCookie = "" + document.cookie
  ind = theCookie.indexOf(cookieName)
  return false if ind == -1 or cookieName == ""
  ind1 = theCookie.indexOf(';', ind)
  ind1 = theCookie.length if ind1 == -1
  value = unescape(theCookie.substring(ind+cookieName.length+1, ind1))
  value

setCookie = (cookieName, cookieValue, msec_in_utc) ->
  expire = new Date(msec_in_utc)
  document.cookie = cookieName + "=" + escape(cookieValue) + ";path=/;expires=" + expire.toUTCString()

today = new Date().getTime()
referrer = if window.decodeURI then window.decodeURI(document.referrer) else document.referrer
landing_page = if window.decodeURI then window.decodeURI(window.location) else window.location
uniq = readCookie('_y')
visit = readCookie('_yy')

if !uniq
  uniq = uniqueId()
  setCookie('_y', uniq, today + (1000*60*60*24*360*20)) # 20 years

if !visit
  (new Image).src = "/s.gif?a=#{uniq}&r=#{encodeURIComponent(referrer)}&l=#{encodeURIComponent(landing_page)}&t=#{today}"

# set return visit cookie, always advance this.
setCookie('_yy', '.', today + (1000*60*30)) # 30 mins

lastSearch = ''

window.t = (e) ->
  val = e.target.value
  if val?.length > 0 and val != lastSearch
    lastSearch = val
    (new Image).src = "/s.gif?a=#{uniq}&s=#{encodeURIComponent(val)}"
